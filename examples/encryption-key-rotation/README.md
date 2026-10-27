# UA 密钥轮换验证

这是独立维护 Job，不属于 Chart，也不是 Helm Hook。只执行存量密文迁移和远端 Webhook Secret 更新，不切换运行中的 AES/Dex Secret。

## 前提

- 已先升级到兼容的业务镜像和 Chart，仍使用原 AES/Dex Secret。配套 Chart 必须包含 [helm-charts PR #477](https://github.com/koderover/helm-charts/pull/477)（提交 `324dc9eef2fe506cb6c6b5f18373b88c8ca4bb3b`）的密钥保留及校验逻辑；不能直接使用未包含该改动的旧 Chart。
- 关闭业务入口、工作流/定时触发/Webhook 消费，结束运行任务，停止 user、aslan、hub-server 等密文读写服务。
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
ZADIG_CHART='<包含 PR #477 改动的 Chart 目录或 tgz 路径>'
```

停写并完成前述数据库等最终备份后，保存当前 Secret 和完整 Helm values。以下目录包含密钥，按客户密钥管理要求保管；不要开启 shell 的 `set -x`。

```bash
umask 077
ROTATION_DIR=$(mktemp -d ./zadig-key-rotation.XXXXXX)
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.before.json"
helm get values "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" --all -o json > "$ROTATION_DIR/values.before.json"
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

有 Hook ID 但没有匹配代码源的孤儿记录默认失败。只有确认能按人工清单处理后，才在 Job 的 `args` 中显式加入 `--allow-orphan-webhooks`；这类记录将携带 `hook_id` 出现在 `manual_webhooks` 中。该参数不会跳过权限不足、Token 失效等候选查询失败。dry-run 和正式 Job 必须使用相同配置。

确认 dry-run 成功且仍在停写状态后，复制 `job.yaml` 为 `job-apply.yaml`，将 Job 名改为 `zadig-key-rotation-apply`，删除 `--dry-run` 这一行，其余配置保持一致。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" create -f job-apply.yaml
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" logs -f job/zadig-key-rotation-apply
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get job zadig-key-rotation-apply
```

正式成功报告为 `phase: completed`，所有 `failed` 为 0。S3、Agent、MFA 和托管 Webhook 的 `succeeded + skipped` 应等于 `total`。按报告手工更新无 Hook ID 的 Webhook 和显式转人工的孤儿记录，其 Secret 为新 AES key 的 SHA-256 十六进制值。

**任何写入失败后保持维护状态，先恢复最终 MongoDB/MySQL 备份，再修复并重跑完整 Job。禁止直接在已迁移或部分迁移的数据上重跑；Job 不会自动重试。** 已更新的远端 Webhook 可重复设置相同新 Secret；放弃轮换时需按备份恢复旧 Webhook Secret。

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

确认整个命令块成功后，准备 Dex values：只替换第一个静态客户端的 Secret，保留所有客户端的 ID、回调地址及其他配置。新 Secret 保存在受限文件中，不输出到终端。

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

确认文件生成成功，再执行第二次 Helm upgrade。`--set-string global.encryption.key=` 清除 release 中历史 key 值，使配套 Chart 读取已切换的当前 Secret。复用客户现有 values，不替换其他镜像或配置。

```bash
helm upgrade "$ZADIG_RELEASE" "$ZADIG_CHART" \
  --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" \
  --reuse-values -f "$ROTATION_DIR/dex-next-values.json" \
  --set-string global.encryption.key=
```

Helm 成功后，校验当前 Secret 的 SHA-256 指纹与新密钥文件一致（仅输出校验结果）。不一致时保持维护状态，中止恢复服务。

```bash
(
set -eu
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get secret zadig-aes-key -o json > "$ROTATION_DIR/aes.after.json"
python3 - "$ROTATION_DIR" <<'PY'
import base64
import hashlib
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
actual = base64.b64decode(json.loads((folder / "aes.after.json").read_text())["data"]["aesKey"], validate=True)
expected = (folder / "aes-next").read_bytes()
assert hashlib.sha256(actual).digest() == hashlib.sha256(expected).digest(), "AES fingerprint mismatch; keep maintenance mode"
print("AES fingerprint matches")
PY
)
```

校验成功后，按停写前记录恢复各服务副本数，并重启确保加载新密钥：

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout restart deployment/user deployment/aslan deployment/hub-server
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/user --timeout=10m
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/aslan --timeout=10m
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status deployment/hub-server --timeout=10m
```

确认 Dex、cron 也已就绪；验收 Agent 重连、原 TOTP 登录、对象存储读写、Webhook 触发、新旧 API Token 和 Dex 授权回调后恢复业务。全部 Agent 完成升级并验证后，按方案删除过渡 Secret。

JWT、API Token、MFA 恢复码将失效，用户需要重新登录/签发；本 Job 不删除其哈希。历史 pipeline `storage_uri` 只计数、不重写。

## 限制与开发验证

旧密文使用无认证的 AES-CFB，S3 Secret 单独无法验证旧 key 是否正确。必须直接挂载当前 Secret，不能靠“解密无错误”判断 key 正确，也不能把工具当成可重复执行的迁移器。GitHub 按现有公共 API 客户端工作；GitLab/Gitee 使用记录中的实例地址。

本 PR 不提交测试文件，不提供依赖这些文件的测试命令。可构建 UA：

```bash
go build ./cmd/ua
```

本地隔离测试不能替代真实代码源验收。[Gitee 官方 PATCH 文档](https://gitee.com/api/v5/swagger)将事件字段标为可选，`push_events` 标注默认 `true`，未承诺省略字段会保留原值；当前 Secret 更新仅发送 URL/password，尚未完成真实 Gitee/Gitee EE 的事件开关前后对比，不应视为已通过验证。正式交付前须在专用测试仓库验证事件配置保持不变、Webhook 仍能触发，并完成客户代理/证书和完整维护窗口验收。
