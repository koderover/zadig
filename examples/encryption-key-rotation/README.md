# 加密密钥轮换

本文说明如何更换 Zadig 的 AES 加密密钥和 Dex 客户端密钥。适用于仍在使用公开默认密钥的环境，或怀疑密钥已经泄露的环境。

UA 的 `rotate-encryption-key` 命令以 Kubernetes Job 的形式在维护窗口内单独运行，它会做两件事：把数据库中已有的密文改用新密钥重新加密，并更新代码源上 Webhook 的 Secret。正式迁移成功后，手动启动一次 `finish-rotation.sh`，由它自动切换 AES Secret、同步 Helm values、更换 Dex 密钥并恢复服务；业务验收和恢复入口仍由人工完成。

## 前提

- 已先升级到兼容的业务镜像和 Chart，仍使用原 AES/Dex Secret。配套 Chart 必须为 Zadig v5.1.0 及以上版本，该版本起升级时始终沿用集群 Secret 中的密钥、忽略 values 中的 `global.encryption.key`；不能使用 v5.1.0 之前的 Chart，否则后续升级可能用 values 中的旧密钥覆盖 Secret。
- 关闭业务入口、工作流/定时触发/Webhook 消费，结束运行任务，停止 user、aslan、hub-server、cron 等密文读写服务；同时暂停会恢复副本数的 HPA/GitOps 等外部控制器。
- 停写后完成 MongoDB、MySQL 用户库和 Dex 库、当前 AES Secret、Helm values、Dex Secret、Webhook 清单和 Agent YAML 的最终备份。
- 本工具只支持 MySQL。代码源 OAuth Token 须在维护前确认有效；dry-run 不刷新 Token、不写数据库或 Redis。
- 使用已发布且支持节点架构的 UA 镜像。客户节点只需正常拉取镜像，不需要 Docker；本地构建/推送工具不属于客户节点依赖。

## 流程概览

```text
阶段一：正常升级到 v5.1.0+ 的镜像和 Chart（仍使用旧密钥）
  │
  ▼
记录副本数，停 user / aslan / hub-server / cron ──► 最终备份（数据库、Secret、Helm values）
  │
  ▼
生成新密钥，创建 zadig-aes-key-next
  │
  ▼
dry-run Job ──► 报告 phase: validated、failed 为 0，处理 manual_webhooks 清单
  │
  ▼
正式 Job：更新远端 Webhook ──► 写 MongoDB / MySQL ──► 清理 Redis MFA challenge
  │            （失败可用同一组密钥重跑）  （失败须先恢复备份）
  ▼
手动启动 finish-rotation.sh（后续步骤自动执行）
  │
  ├─ 创建 previous Secret，切换 AES Secret 并校验
  ├─ 同版本 Helm upgrade --no-hooks：同步 AES values、更换 Dex 密钥
  └─ 恢复服务副本，重启其他 AES 使用方，等待就绪并校验 values
  │
  ▼
人工验收 ──► 恢复业务触发、入口和外部控制器 ──► 删除 zadig-aes-key-next
  │
  ▼
全部 Agent 升级完成后，尽快删除 zadig-aes-key-previous
```

## 执行

以下命令使用当前 kubectl context。先确认目标集群和安装命名空间；命名空间、release 和镜像地址需要替换。以下命令需安装 kubectl、Helm、Python 3 和 OpenSSL，不需要其他组件；Helm 用于查看 release、备份 values，以及最后更换 Dex 客户端密钥。

下面这段确认当前集群，并设置后续命令共用的变量。

```bash
kubectl config current-context
helm list -A
ZADIG_CONTEXT=$(kubectl config current-context)
ZADIG_NAMESPACE='<安装命名空间>'
ZADIG_RELEASE='<现有 Helm release>'
helm history "$ZADIG_RELEASE" --kube-context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE"
```

核对 `helm history` 中当前部署的 Chart 为 v5.1.0 及以上。

关闭业务入口和触发、结束运行任务后，先记录副本数，再停服务。以下目录包含密钥，按客户密钥管理要求保管；不要开启 shell 的 `set -x`。命令中的 Deployment 名称为 Chart 默认名称，自定义部署先核对名称。

脚本作用：
1. 创建权限为 700 的维护目录，把上面的变量写入 `session.sh`，终端断开后可以恢复。
2. 导出四个 Deployment 的当前定义，从中读出原副本数，写入 `replicas.before.json`，最后恢复服务时使用。
3. 把四个服务缩到 0，并等待 Pod 全部退出。

```bash
umask 077
ROTATION_DIR=$(mktemp -d "$PWD/zadig-key-rotation.XXXXXX")
printf '维护目录（请记录绝对路径）: %s\n' "$ROTATION_DIR"
(
set -eu
python3 - "$ROTATION_DIR" "$ZADIG_CONTEXT" "$ZADIG_NAMESPACE" "$ZADIG_RELEASE" <<'PY'
import shlex
import sys
from pathlib import Path

names = ("ROTATION_DIR", "ZADIG_CONTEXT", "ZADIG_NAMESPACE", "ZADIG_RELEASE")
Path(sys.argv[1], "session.sh").write_text("umask 077\n" + "".join(
    f"{name}={shlex.quote(value)}\n" for name, value in zip(names, sys.argv[1:])))
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment user aslan hub-server cron -o json > "$ROTATION_DIR/deployments.before.json"
python3 - "$ROTATION_DIR" <<'PY'
import json
import sys
from pathlib import Path

folder = Path(sys.argv[1])
names = {"user", "aslan", "hub-server", "cron"}
items = json.loads((folder / "deployments.before.json").read_text())["items"]
replicas = {item["metadata"]["name"]: item["spec"]["replicas"] for item in items}
assert set(replicas) == names, "missing maintenance deployment"
(folder / "replicas.before.json").write_text(json.dumps(replicas))
PY
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" scale deployment/user deployment/aslan deployment/hub-server deployment/cron --replicas=0
for service in user aslan hub-server cron; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status "deployment/$service" --timeout=10m
done
)
```

确认整个命令块成功，核对相关 Pod 已退出、没有其他写入者后，完成前述数据库等最终备份，再保存当前 Secret 和完整 Helm values。备份失败时不得继续迁移。

下面这段把当前 `zadig-aes-key` 和 release 的全部 values 导出到维护目录，前者用于构造旧密钥 Secret，后者作为备份。

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

## 迁移完成后的自动收尾

正式 Job 成功、人工 Webhook 清单处理完毕，且四个核心 Deployment 仍为 0 副本、对应 Pod 全部退出时，执行一次收尾脚本：

```bash
ZADIG_CHART='<当前已部署的 Chart，例如 koderover-chart/zadig>'
ZADIG_CHART_VERSION='<helm history 中当前的 Chart 版本>'
bash examples/encryption-key-rotation/finish-rotation.sh \
  "$ROTATION_DIR" "$ZADIG_CONTEXT" "$ZADIG_NAMESPACE" "$ZADIG_RELEASE" \
  "$ZADIG_CHART" "$ZADIG_CHART_VERSION"
```

脚本按顺序完成：

1. 验证正式 Job 已成功、当前 AES Secret 仍是旧 key、`zadig-aes-key-next` 与 `aes-next` 一致，并确认核心服务仍停着。
2. 创建 `zadig-aes-key-previous`，再把 `zadig-aes-key` 切换到新 key 并校验。
3. 使用当前部署的完全相同 Chart 和版本执行 `helm upgrade --no-hooks`，同步显式保存的旧 AES key，并生成新的 Dex 客户端密钥；空值或未配置的 AES key 保持原样。`--no-hooks` 避免重复执行 UA 迁移、初始化和版本记录 Job。
4. Helm 提交成功后恢复维护前记录的副本数（包括原来为 0 的服务），重启其他引用 AES Secret 的 Deployment，等待 Deployment/StatefulSet 就绪，并校验 Helm values。Helm 命令不加 `--wait`，避免在恢复副本前等待；以整个脚本成功退出作为收尾完成标志，不能只看 release 的 `deployed` 状态。

脚本依赖维护目录中的 `aes.before.json`、`aes-next` 和 `replicas.before.json`；正式 Job 使用上述固定名称 `zadig-key-rotation-apply`，请保留 Job 及日志至收尾完成。

使用 Helm 4 时，这次收尾显式加 `--server-side=false`，避免服务缩容产生的字段管理冲突；Helm 3 使用原有客户端更新方式。

脚本仍会更新整个 release，但不改变 Chart 的密钥解析规则：首次安装自动生成，普通升级复用集群 Secret。它只在轮转收尾时手动启动一次；普通 Helm 升级、admin 更新和 UA 迁移 Job 都不会调用它。

脚本会在维护目录创建 `rotation-finish` 保存备份和结果；目录已存在时直接退出，避免重复切换 Dex。失败或中断后保持维护状态，先检查实际资源状态再处理，不能直接重跑。它不自动回滚：Secret 切换后的失败可能已改变 Secret、release 或部分副本数。业务入口、定时触发和外部同步控制器仍需在人工验收后恢复。

验收 Agent 重连、原 TOTP 登录、对象存储读写、Webhook、API Token 和 Dex 授权回调。验收通过后删除临时 Secret `zadig-aes-key-next`；全部 Agent 切换完成后再删除 `zadig-aes-key-previous`。维护目录中的明文 key 和 values 文件按客户密钥管理要求保管或销毁。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" delete secret zadig-aes-key-next
# 仅在全部 Agent 完成切换和验收后执行：
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" delete secret zadig-aes-key-previous
```

JWT、API Token、MFA 恢复码将失效，用户需要重新登录/签发；本 Job 不删除其哈希。历史 pipeline `storage_uri` 只计数、不重写。

## 限制与开发验证

旧密文使用无认证的 AES-CFB，S3 Secret 单独无法验证旧 key 是否正确。必须直接挂载当前 Secret，不能靠“解密无错误”判断 key 正确，也不能把工具当成可重复执行的迁移器。GitHub 按现有公共 API 客户端工作；GitLab/Gitee 使用记录中的实例地址。

本 PR 不提交测试文件，不提供依赖这些文件的测试命令。可构建 UA：

```bash
go build ./cmd/ua
```

本地隔离测试不能替代真实代码源验收。[Gitee 官方 PATCH 文档](https://gitee.com/api/v5/swagger)将事件字段标为可选，`push_events` 标注默认 `true`，未承诺省略字段会保留原值；当前 Secret 更新仅发送 URL/password，尚未完成真实 Gitee/Gitee EE 的事件开关前后对比，不应视为已通过验证。正式交付前须在专用测试仓库验证事件配置保持不变、Webhook 仍能触发，并完成客户代理/证书和完整维护窗口验收。
