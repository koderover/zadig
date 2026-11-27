#!/bin/bash
# Run after the migration Job succeeds and manual Webhooks have been updated.
set -euo pipefail
umask 077

if [ "$#" -ne 6 ]; then
  printf 'Usage: bash %s <rotation-dir> <context> <namespace> <release> <chart> <chart-version>\n' "$0" >&2
  exit 1
fi
ROTATION_DIR=$1
ZADIG_CONTEXT=$2
ZADIG_NAMESPACE=$3
ZADIG_RELEASE=$4
ZADIG_CHART=$5
ZADIG_CHART_VERSION=$6
HELM_OPTIONS=(--no-hooks --timeout 10m)
# Helm 4 SSA conflicts with replica changes made during maintenance.
HELM_VERSION=$(helm version --short)
case "$HELM_VERSION" in
  v4.*) HELM_OPTIONS+=(--server-side=false) ;;
esac

# Keep the first attempt intact, including after a failed or interrupted upgrade.
FINISH_DIR="$ROTATION_DIR/rotation-finish"
if ! mkdir -m 700 "$FINISH_DIR"; then
  printf 'Cannot create finish directory; inspect the previous attempt before retrying: %s\n' "$FINISH_DIR" >&2
  exit 1
fi

helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" -o json > "$FINISH_DIR/values.current.json"
helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" --all -o json > "$FINISH_DIR/values.computed.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get job zadig-key-rotation-apply -o json > "$FINISH_DIR/job.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" logs job/zadig-key-rotation-apply > "$FINISH_DIR/job.log"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment -o json > "$FINISH_DIR/deployments.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$FINISH_DIR/aes.current.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key-next -o json > "$FINISH_DIR/aes.next.json"
python3 - "$FINISH_DIR" <<'PY'
import base64
import json
import secrets
import sys
from pathlib import Path

folder = Path(sys.argv[1])
values = json.loads((folder / "values.current.json").read_text()) or {}
old = base64.b64decode(json.loads((folder.parent / "aes.before.json").read_text())["data"]["aesKey"], validate=True).decode()
new = (folder.parent / "aes-next").read_text()
current = json.loads((folder / "aes.current.json").read_text())
actual = base64.b64decode(current["data"]["aesKey"], validate=True).decode()
next_key = json.loads((folder / "aes.next.json").read_text())["data"]["aesKey"]
if actual != old or len(new.encode()) not in (16, 24, 32) or new == old:
    sys.exit("current AES key must match the backup and differ from the valid new key")
if base64.b64decode(next_key, validate=True).decode() != new:
    sys.exit("next AES Secret does not match the migration key file")
job = json.loads((folder / "job.json").read_text())
if not any(item["type"] == "Complete" and item["status"] == "True" for item in job.get("status", {}).get("conditions", [])):
    sys.exit("migration Job has not completed")
report = json.loads((folder / "job.log").read_text().strip().splitlines()[-1])
if report.get("phase") != "completed" or any(item["failed"] for item in report["records"].values()):
    sys.exit("migration report is not successful")
replicas = json.loads((folder.parent / "replicas.before.json").read_text())
names = {"user", "aslan", "hub-server", "cron"}
if set(replicas) != names or any(type(count) is not int or count < 0 for count in replicas.values()):
    sys.exit("invalid original replica counts")
deployments = json.loads((folder / "deployments.json").read_text())["items"]
core = {item["metadata"]["name"]: item for item in deployments if item["metadata"]["name"] in names}
if set(core) != names or any(item["spec"]["replicas"] != 0 for item in core.values()):
    sys.exit("keep user, aslan, hub-server and cron scaled to zero before switching keys")
(folder / "replicas.txt").write_text("".join(f"{name} {count}\n" for name, count in replicas.items()))
(folder / "restart.txt").write_text("".join(
    item["metadata"]["name"] + "\n" for item in deployments
    if item["metadata"]["name"] not in names and '"zadig-aes-key"' in json.dumps(item["spec"]["template"])))
previous = {"apiVersion": "v1", "kind": "Secret", "metadata": {
    "name": "zadig-aes-key-previous", "namespace": current["metadata"]["namespace"]},
    "type": "Opaque", "data": {"aesKey": current["data"]["aesKey"]}}
(folder / "aes.previous.json").write_text(json.dumps(previous))
(folder / "aes.patch.json").write_text(json.dumps({
    "metadata": {"resourceVersion": current["metadata"]["resourceVersion"]}, "data": {"aesKey": next_key}}))
overrides = {}
configured = values.get("global", {}).get("encryption", {}).get("key")
if configured and configured != new:
    if configured != old:
        sys.exit("configured AES key is neither the old nor the new key")
    overrides["global"] = {"encryption": {"key": new}}
clients = json.loads((folder / "values.computed.json").read_text())["dex"]["config"]["staticClients"]
if not clients or not clients[0]["id"]:
    sys.exit("missing Zadig Dex client")
clients[0]["secret"] = secrets.token_hex(32)
overrides["dex"] = {"config": {"staticClients": clients}}
(folder / "rotation-values.json").write_text(json.dumps(overrides))
PY

kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create -f "$FINISH_DIR/aes.previous.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" patch secret zadig-aes-key --type merge --patch-file "$FINISH_DIR/aes.patch.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$FINISH_DIR/aes.switched.json"
python3 - "$FINISH_DIR" <<'PY'
import base64
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
actual = base64.b64decode(json.loads((folder / "aes.switched.json").read_text())["data"]["aesKey"], validate=True)
if actual != (folder.parent / "aes-next").read_bytes():
    sys.exit("AES Secret switch verification failed; keep maintenance mode")
print("AES Secret switch verified")
PY

# Restore recorded replica counts after applying the Chart, then wait for rollouts.
helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" --version "$ZADIG_CHART_VERSION" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  -f "$FINISH_DIR/values.current.json" -f "$FINISH_DIR/rotation-values.json" \
  "${HELM_OPTIONS[@]}" > "$FINISH_DIR/helm-dex.out"

while read -r service count; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" scale "deployment/$service" --replicas="$count"
done < "$FINISH_DIR/replicas.txt"

while read -r service; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout restart "deployment/$service"
done < "$FINISH_DIR/restart.txt"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment,statefulset -o name > "$FINISH_DIR/workloads.txt"
while read -r workload; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status "$workload" --timeout=10m
done < "$FINISH_DIR/workloads.txt"

helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" -o json > "$FINISH_DIR/values.after.json"
python3 - "$FINISH_DIR" <<'PY'
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
before = json.loads((folder / "values.current.json").read_text()) or {}
after = json.loads((folder / "values.after.json").read_text())
new = (folder.parent / "aes-next").read_text()
configured = before.get("global", {}).get("encryption", {}).get("key")
expected = new if configured else configured
if after.get("global", {}).get("encryption", {}).get("key") != expected:
    sys.exit("unexpected AES key in Helm values")
clients = json.loads((folder / "rotation-values.json").read_text())["dex"]["config"]["staticClients"]
actual = {client["id"]: client["secret"] for client in after["dex"]["config"]["staticClients"]}
if actual.get(clients[0]["id"]) != clients[0]["secret"]:
    sys.exit("Helm values contain a different Dex client secret")
print("Rotation finish verified; keep business triggers paused until acceptance")
PY
