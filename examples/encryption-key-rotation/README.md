# UA 密钥轮换验证

这是独立维护 Job，不属于 Chart，也不是 Helm Hook。只执行存量密文迁移和远端 Webhook Secret 更新，不切换运行中的 AES/Dex Secret。

## 前提

- 已先升级到兼容的业务镜像和 Chart，仍使用原 AES/Dex Secret。配套 Chart 必须包含 [helm-charts PR #477](https://github.com/koderover/helm-charts/pull/477)（提交 `324dc9eef2fe506cb6c6b5f18373b88c8ca4bb3b`）的密钥保留及校验逻辑；不能直接使用未包含该改动的旧 Chart。
- 关闭业务入口、工作流/定时触发/Webhook 消费，结束运行任务，停止 user、aslan、hub-server、cron 等密文读写服务；同时暂停会恢复副本数的 HPA/GitOps 等外部控制器。
- 停写后完成 MongoDB、MySQL 用户库和 Dex 库、当前 AES Secret、Helm values、Dex Secret、Webhook 清单和 Agent YAML 的最终备份。
- 本工具只支持 MySQL。代码源 OAuth Token 须在维护前确认有效；dry-run 不刷新 Token、不写数据库或 Redis。
- 使用已发布且支持节点架构的 UA 镜像。客户节点只需正常拉取镜像，不需要 Docker；本地构建/推送工具不属于客户节点依赖。

## 执行

以下命令使用当前 kubectl context。先确认目标集群和安装命名空间；命名空间、release、Chart 路径和镜像地址需要替换。以下命令需安装 kubectl、Helm、Python 3 和 OpenSSL。

```bash
kubectl config current-context
helm list -A
ZADIG_CONTEXT=$(kubectl config current-context)
ZADIG_NAMESPACE='<安装命名空间>'
ZADIG_RELEASE='<现有 Helm release>'
ZADIG_CHART='<阶段一已部署的同一份 Chart 目录或 tgz 的绝对路径>'
helm history "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE"
```

第二次及后续 upgrade 必须使用阶段一的同一份 Chart（包含 PR #477 改动），同时核对 `helm history` 中的版本；相同版本号不保证包内容一致。更换 Chart 可能改变依赖 `.Chart.AppVersion` 的镜像，`--reuse-values` 不能保证镜像不变。

关闭业务入口和触发、结束运行任务后，先记录副本数，再停服务。以下目录包含密钥，按客户密钥管理要求保管；不要开启 shell 的 `set -x`。命令中的 Deployment 名称为 Chart 默认名称，自定义部署先核对名称。

```bash
umask 077
ROTATION_DIR=$(mktemp -d "$PWD/zadig-key-rotation.XXXXXX")
printf '维护目录（请记录绝对路径）: %s\n' "$ROTATION_DIR"
(
set -eu
python3 - "$ROTATION_DIR" "$ZADIG_CONTEXT" "$ZADIG_NAMESPACE" "$ZADIG_RELEASE" "$ZADIG_CHART" <<'PY'
import shlex
import sys
from pathlib import Path

names = ("ROTATION_DIR", "ZADIG_CONTEXT", "ZADIG_NAMESPACE", "ZADIG_RELEASE", "ZADIG_CHART")
Path(sys.argv[1], "session.sh").write_text("umask 077\n" + "".join(
    f"{name}={shlex.quote(value)}\n" for name, value in zip(names, sys.argv[1:])))
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment user aslan hub-server cron -o json > "$ROTATION_DIR/deployments.before.json"
python3 - "$ROTATION_DIR" <<'PY'
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
names = {"user": "user", "aslan": "aslan", "hub-server": "hubServer", "cron": "cron"}
items = json.loads((folder / "deployments.before.json").read_text())["items"]
replicas = {names[item["metadata"]["name"]]: {"replicas": str(item["spec"]["replicas"])} for item in items}
assert set(replicas) == set(names.values()), "missing maintenance deployment"
(folder / "replicas.before-values.json").write_text(json.dumps({"microservice": replicas}))
replicas["cron"]["replicas"] = "0"
(folder / "services-resume-values.json").write_text(json.dumps({"microservice": replicas}))
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" scale deployment/user deployment/aslan deployment/hub-server deployment/cron --replicas=0
for service in user aslan hub-server cron; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status "deployment/$service" --timeout=10m
done
)
```

确认整个命令块成功，核对相关 Pod 已退出、没有其他写入者后，完成前述数据库等最终备份，再保存当前 Secret 和完整 Helm values。备份失败时不得继续迁移。

```bash
(
set -eu
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.before.json"
helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" --all -o json > "$ROTATION_DIR/values.before.json"
)
```

维护中断或终端重连时，从已记录的实际目录恢复变量；只加载本次维护生成的文件。**恢复变量不表示任何步骤已成功，不要从头重跑迁移或重新生成新密钥。**

```bash
source /绝对路径/zadig-key-rotation.XXXXXX/session.sh
```

生成 32 字节的新 AES key（文件无换行），创建临时 Secret。文件按客户密钥管理要求保管，不提交 Git。

```bash
umask 077
openssl rand -hex 16 | tr -d '\n' > "$ROTATION_DIR/aes-next"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create secret generic zadig-aes-key-next --from-file=aesKey="$ROTATION_DIR/aes-next"
```

修改 `job.yaml` 中的镜像为本次发布的 UA 完整地址。私有仓库在 Pod spec 添加已有的 `imagePullSecrets`。Job 从现有 `user-config` 读取 MongoDB、MySQL、Redis 配置（包括 `REDIS_USER_TOKEN_DB`）；代码源代理按数据库中现有配置读取。核对该 ConfigMap 与当前部署一致；使用自定义证书的代码源须挂载客户 CA 并设置 `SSL_CERT_FILE`。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create -f job.yaml
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" logs -f job/zadig-key-rotation-dry-run
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get job zadig-key-rotation-dry-run
```

成功报告应为 `phase: validated`，各类 `failed` 为 0；dry-run 的 `succeeded` 为 0，解密通过数量见 `validated`。检查 `manual_webhooks`，记录需要人工更新的仓库。

同地址存在多个代码源时，按代码源 ID 升序逐个用现有 Token 只读查询该仓库的 Hook ID，选择第一个查询成功的候选；所有候选均失败时，输出候选代码源 ID、名称和失败原因并中止。**可读不代表可修改，dry-run 无法保证写权限**；维护前应确认候选 Token 具备 Hook 管理权限。

有 Hook ID 但没有匹配代码源的孤儿记录默认失败。只有确认能按人工清单处理后，才在 Job 的 `args` 中显式加入 `--allow-orphan-webhooks`；这类记录将携带 `hook_id` 出现在 `manual_webhooks` 中。该参数不会跳过权限不足、Token 失效等候选查询失败。同地址候选均不可读时，应先修正 Token 或权限；只有确认记录已废弃、备份并核实远端 Hook 后，才清理对应记录，不能删除仍有效的记录绕过检查。dry-run 和正式 Job 必须使用相同配置。

确认 dry-run 成功且仍在停写状态后，复制 `job.yaml` 为 `job-apply.yaml`，将 Job 名改为 `zadig-key-rotation-apply`，删除 `--dry-run` 这一行，其余配置保持一致。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create -f job-apply.yaml
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" logs -f job/zadig-key-rotation-apply
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get job zadig-key-rotation-apply
```

正式成功报告为 `phase: completed`，所有 `failed` 为 0。S3、Agent、MFA 和托管 Webhook 的 `succeeded + skipped` 应等于 `total`。按报告手工更新无 Hook ID 的 Webhook 和显式转人工的孤儿记录，其 Secret 为新 AES key 的 SHA-256 十六进制值。

正式执行先更新远端托管 Webhook，再写 MongoDB/MySQL 并清理 Redis MFA challenge。所有远端查询和更新均受命令取消信号及单次 30 秒请求超时约束；Job 不会自动重试。

- 报告停在 `phase: updating_webhooks`：本次尚未修改 MongoDB、MySQL 或 Redis。保持维护状态，修正远端权限/连接后，使用**同一组旧、新密钥**重跑完整 Job，无需恢复数据库；部分远端 Hook 可能已更新或结果不确定，可重复设置相同新 Secret。放弃轮换时须按备份恢复已更新的远端 Hook Secret。
- 报告停在 `phase: writing`：可能已写入部分本地数据。**先恢复最终 MongoDB/MySQL 备份，再修复并重跑完整 Job。禁止直接在已迁移或部分迁移的数据上重跑。**
- Job 超过 `activeDeadlineSeconds: 1800` 被终止、报告缺失或无法确认失败阶段时，按可能已写库处理，保持维护状态并恢复最终备份后再重跑。

## 迁移后的步骤

仅在正式 Job 成功、人工清单处理完成且仍处于维护状态时执行。以下命令从最终备份构造旧密钥 Secret，从临时 Secret 读取新密钥；先创建 `zadig-aes-key-previous`，成功后才修改当前 Secret。已有 previous Secret 时创建会失败，需先核实其来源，不能直接覆盖。

```bash
(
set -eu
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key-next -o json > "$ROTATION_DIR/aes.next.json"
python3 - "$ROTATION_DIR" "$ZADIG_NAMESPACE" <<'PY'
import base64
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
old = json.loads((folder / "aes.before.json").read_text())["data"]["aesKey"]
new = json.loads((folder / "aes.next.json").read_text())["data"]["aesKey"]
assert len(base64.b64decode(new, validate=True)) in (16, 24, 32), "invalid new key"
assert base64.b64decode(old) != base64.b64decode(new), "keys must differ"
previous = {"apiVersion": "v1", "kind": "Secret", "metadata": {
    "name": "zadig-aes-key-previous", "namespace": sys.argv[2]},
    "type": "Opaque", "data": {"aesKey": old}}
(folder / "aes.previous.json").write_text(json.dumps(previous))
(folder / "aes.patch.json").write_text(json.dumps({"data": {"aesKey": new}}))
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create -f "$ROTATION_DIR/aes.previous.json"
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" patch secret zadig-aes-key --type merge --patch-file "$ROTATION_DIR/aes.patch.json"
)
```

Secret 切换成功后，先校验当前 Secret 的 SHA-256 指纹与本次迁移使用的新密钥文件一致。将校验脚本保存在维护目录，供 Helm 前后复用；仅输出校验结果。整个命令块必须成功才可继续，不一致时保持服务为 0，中止后续步骤。

```bash
(
set -eu
cat > "$ROTATION_DIR/check-aes.py" <<'PY'
import base64
import hashlib
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
actual = base64.b64decode(json.loads((folder / "aes.current.json").read_text())["data"]["aesKey"], validate=True)
expected = (folder / "aes-next").read_bytes()
assert hashlib.sha256(actual).digest() == hashlib.sha256(expected).digest(), "AES fingerprint mismatch; keep maintenance mode"
print("AES fingerprint matches")
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.current.json"
python3 "$ROTATION_DIR/check-aes.py" "$ROTATION_DIR"
)
```

确认校验成功后，准备 Dex values：只替换第一个静态客户端的 Secret，保留所有客户端的 ID、回调地址及其他配置。新 Secret 保存在受限文件中，不输出到终端。

```bash
python3 - "$ROTATION_DIR" <<'PY'
import json
import secrets
import sys
from pathlib import Path

folder = Path(sys.argv[1])
clients = json.loads((folder / "values.before.json").read_text())["dex"]["config"]["staticClients"]
assert clients and clients[0]["id"], "missing Zadig Dex client"
clients[0]["secret"] = secrets.token_hex(32)
(folder / "dex-next-values.json").write_text(json.dumps({"dex": {"config": {"staticClients": clients}}}))
PY
```

确认文件生成成功，再执行第二次 Helm upgrade。`--set-string global.encryption.key=` 清除 release 中历史 key 值，使配套 Chart 通过 lookup 读取已校验的当前 Secret。使用同一份 Chart 并复用现有 values。

**Helm upgrade 会把手工 scale 为 0 的服务恢复到 Chart 指定的副本数。** 停写升级使用以下 Helm 3 post-renderer，将四个 Deployment 的副本数保持为 0；它通过已有的 `kubectl kustomize` 处理本次渲染结果，不连接集群。不通过 values 将 Aslan 副本数设为 0：多副本 Aslan 的 `aslan-pvc` 和 `/app/data` 挂载依赖该值，设为 0 会使 PVC 从模板中消失并被 Helm 删除。保留原 values 后，这些资源继续正常渲染。

```bash
(
set -eu
mkdir "$ROTATION_DIR/helm-maintenance"
cat > "$ROTATION_DIR/helm-maintenance/kustomization.yaml" <<'YAML'
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - rendered.yaml
replicas:
  - name: user
    count: 0
  - name: aslan
    count: 0
  - name: hub-server
    count: 0
  - name: cron
    count: 0
YAML
cat > "$ROTATION_DIR/helm-maintenance/render.sh" <<'SH'
#!/bin/sh
set -eu
cd "$(dirname "$0")"
cat > rendered.yaml
kubectl kustomize .
SH
chmod 700 "$ROTATION_DIR/helm-maintenance/render.sh"
)
```

本次停写升级必须加 `--no-hooks`：Chart 的 init hook 会通过 HTTP 调用 Aslan，Aslan 为 0 时会失败；同样跳过每次升级都会执行的 UA migrate hook。阶段一已完成正常升级和 hooks，独立轮换 Job 不受 `--no-hooks` 影响。不要在这里使用 `--atomic`，以免失败时自动回滚并启动服务。

```bash
helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  --reuse-values -f "$ROTATION_DIR/dex-next-values.json" \
  --set-string global.encryption.key= \
  --post-renderer "$ROTATION_DIR/helm-maintenance/render.sh" \
  --no-hooks
```

Helm 成功后再次复核指纹。校验失败时保持维护状态，不执行任何恢复服务的命令。

```bash
(
set -eu
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.current.json"
python3 "$ROTATION_DIR/check-aes.py" "$ROTATION_DIR"
)
```

两次指纹校验及第二次 Helm upgrade 均成功后，用停写前记录生成的文件恢复 user、aslan、hub-server 的副本数，cron 继续保持为 0。文件只覆盖副本数，不回填旧 AES/Dex values。此次不再指定 post-renderer 或 `--no-hooks`，恢复正常 init 和 UA migrate hooks；使用 `--wait --timeout 10m` 等待服务就绪后执行 post-upgrade hooks。已停止的 Pod 会重新创建并加载新密钥，无需额外 rollout restart。每次 upgrade 成功后才执行后续步骤。

```bash
helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  --reuse-values -f "$ROTATION_DIR/services-resume-values.json" \
  --wait --timeout 10m
```

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/user --timeout=10m
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/aslan --timeout=10m
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/hub-server --timeout=10m
```

确认 Dex 就绪，验收 Agent 重连、原 TOTP 登录、对象存储读写、受控 Webhook 触发、新旧 API Token 和 Dex 授权回调。**业务定时触发配置继续保持暂停**，不要因为服务就绪就恢复业务流量。

验收通过后，用原始副本数文件恢复 cron，将 release 中此次维护的临时副本数改回维护前记录的值；原副本数为 0 的服务仍保持为 0。cron 通过 values 保持为字符串 `"0"`，因为模板的 `default 1` 会将数字 `0` 变成 `1`。确认恢复结果，再按维护前记录恢复业务定时触发、入口和外部控制器。

```bash
helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  --reuse-values -f "$ROTATION_DIR/replicas.before-values.json" \
  --wait --timeout 10m
```

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/cron --timeout=10m
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment user aslan hub-server cron
```

全部 Agent 完成升级并验证后，按方案删除过渡 Secret。

JWT、API Token、MFA 恢复码将失效，用户需要重新登录/签发；本 Job 不删除其哈希。历史 pipeline `storage_uri` 只计数、不重写。

## 限制与开发验证

旧密文使用无认证的 AES-CFB，S3 Secret 单独无法验证旧 key 是否正确。必须直接挂载当前 Secret，不能靠“解密无错误”判断 key 正确，也不能把工具当成可重复执行的迁移器。GitHub 按现有公共 API 客户端工作；GitLab/Gitee 使用记录中的实例地址。

本 PR 不提交测试文件，不提供依赖这些文件的测试命令。可构建 UA：

```bash
go build ./cmd/ua
```

本地隔离测试不能替代真实代码源验收。[Gitee 官方 PATCH 文档](https://gitee.com/api/v5/swagger)将事件字段标为可选，`push_events` 标注默认 `true`，未承诺省略字段会保留原值；当前 Secret 更新仅发送 URL/password，尚未完成真实 Gitee/Gitee EE 的事件开关前后对比，不应视为已通过验证。正式交付前须在专用测试仓库验证事件配置保持不变、Webhook 仍能触发，并完成客户代理/证书和完整维护窗口验收。
