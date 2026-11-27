#!/bin/bash
# Run only after the AES migration and Secret switch have been verified.
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

helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" -o json > "$ROTATION_DIR/values.current.json"
helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" --all -o json > "$ROTATION_DIR/values.computed.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.current.json"
python3 - "$ROTATION_DIR" <<'PY'
import base64
import json
import secrets
import sys
from pathlib import Path

folder = Path(sys.argv[1])
values = json.loads((folder / "values.current.json").read_text()) or {}
old = base64.b64decode(json.loads((folder / "aes.before.json").read_text())["data"]["aesKey"], validate=True).decode()
new = (folder / "aes-next").read_text()
actual = base64.b64decode(json.loads((folder / "aes.current.json").read_text())["data"]["aesKey"], validate=True).decode()
if actual != new:
    sys.exit("AES Secret does not match the rotated key")
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

helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" --version "$ZADIG_CHART_VERSION" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  -f "$ROTATION_DIR/values.current.json" -f "$ROTATION_DIR/rotation-values.json" \
  --wait --timeout 10m > "$ROTATION_DIR/helm-dex.out"

helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" -o json > "$ROTATION_DIR/values.after.json"
python3 - "$ROTATION_DIR" <<'PY'
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
before = json.loads((folder / "values.current.json").read_text()) or {}
after = json.loads((folder / "values.after.json").read_text())
new = (folder / "aes-next").read_text()
configured = before.get("global", {}).get("encryption", {}).get("key")
expected = new if configured else configured
if after.get("global", {}).get("encryption", {}).get("key") != expected:
    sys.exit("unexpected AES key in Helm values")
clients = json.loads((folder / "rotation-values.json").read_text())["dex"]["config"]["staticClients"]
actual = {client["id"]: client["secret"] for client in after["dex"]["config"]["staticClients"]}
if actual.get(clients[0]["id"]) != clients[0]["secret"]:
    sys.exit("Helm values contain a different Dex client secret")
print("Helm key values verified")
PY
