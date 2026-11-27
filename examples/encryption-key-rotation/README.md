# 加密密钥轮换

本文说明如何更换 Zadig 的 AES 加密密钥和 Dex 客户端密钥。适用于仍在使用公开默认密钥的环境，或怀疑密钥已经泄露的环境。

UA 的 `rotate-encryption-key` 命令以 Kubernetes Job 的形式在维护窗口内单独运行，它会做两件事：把数据库中已有的密文改用新密钥重新加密，并更新代码源上 Webhook 的 Secret。集群中的密钥 Secret 不由这个命令修改，需要按下文步骤用 kubectl 手动切换，这部分不需要执行 Helm upgrade。AES 密钥切换并验收后，再用一次 Helm upgrade 替换 values 中显式保存的旧 AES 密钥并更换 Dex 客户端密钥。

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
创建 zadig-aes-key-previous（旧密钥），当前 Secret 切到新密钥 ──► 指纹校验
  │
  ▼
kubectl 恢复四个服务的原副本数，重启其他引用该 Secret 的服务 ──► 验收 ──► 恢复业务触发、入口和外部控制器
  │
  ▼
删除 zadig-aes-key-next
  │
  ▼
通过 Helm 同步显式旧 AES values、更换 Dex 客户端密钥 ──► 验收登录
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

## 迁移后的步骤

仅在正式 Job 成功、人工清单处理完成且仍处于维护状态时执行。以下命令从最终备份构造旧密钥 Secret，从临时 Secret 读取新密钥；先创建 `zadig-aes-key-previous`，成功后才修改当前 Secret。已有 previous Secret 时创建会失败，需先核实其来源，不能直接覆盖。

脚本作用：
1. 从 `aes.before.json` 读旧密钥，从 `zadig-aes-key-next` 读新密钥，校验新密钥长度为 16/24/32 字节且与旧密钥不同。
2. 生成 `zadig-aes-key-previous` 的 Secret 定义（内容为旧密钥），以及把 `zadig-aes-key` 改为新密钥的 patch 文件。
3. 先创建 previous Secret，成功后再 patch 当前 Secret。

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

Secret 切换成功后，先校验当前 Secret 的 SHA-256 指纹与本次迁移使用的新密钥文件一致。校验脚本保存在维护目录，仅输出校验结果。整个命令块必须成功才可继续，不一致时保持服务为 0，中止后续步骤。

脚本作用：在维护目录写入 `check-aes.py`，它比较集群中 `zadig-aes-key` 的值与本地 `aes-next` 文件的 SHA-256，一致时只输出 `AES fingerprint matches`，不打印密钥本身；然后导出当前 Secret 并执行一次校验。

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

确认校验成功后，用停写前记录的副本数把四个服务恢复到维护前的状态；原副本数为 0 的服务仍保持为 0。新建的 Pod 会加载新密钥。

```bash
(
set -eu
python3 - "$ROTATION_DIR" <<'PY' | while read -r service count; do
import json
import sys
from pathlib import Path

for name, count in json.loads((Path(sys.argv[1]) / "replicas.before.json").read_text()).items():
    print(name, count)
PY
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" scale "deployment/$service" --replicas="$count"
done
for service in user aslan hub-server cron; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status "deployment/$service" --timeout=10m
done
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment user aslan hub-server cron
)
```

其他 Deployment 若也通过环境变量引用 `zadig-aes-key`（例如 halman），运行中的 Pod 仍持有旧密钥，需要重启。下面这段列出这些 Deployment（不含上述四个服务）并逐个重启。

```bash
(
set -eu
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" get deployment -o json > "$ROTATION_DIR/deployments.all.json"
python3 - "$ROTATION_DIR" <<'PY' > "$ROTATION_DIR/restart.txt"
import json
import sys
from pathlib import Path

skip = {"user", "aslan", "hub-server", "cron"}
for item in json.loads((Path(sys.argv[1]) / "deployments.all.json").read_text())["items"]:
    if item["metadata"]["name"] not in skip and '"zadig-aes-key"' in json.dumps(item["spec"]["template"]):
        print(item["metadata"]["name"])
PY
while read -r service; do
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout restart "deployment/$service"
  kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" rollout status "deployment/$service" --timeout=10m
done < "$ROTATION_DIR/restart.txt"
)
```

普通升级不轮转密钥，也不需要回写 values。本次真正轮转完成后，在下面更换 Dex 密钥的步骤中同步 values：`global.encryption.key` 原来为空或未配置时保持原样，原来保存了旧密钥时才替换为新密钥。在线 Helm 升级虽会优先读取当前 Secret，但离线渲染会使用 values 中的显式值，不能继续保留旧密钥作为后续部署输入。

验收 Agent 重连、原 TOTP 登录、对象存储读写、受控 Webhook 触发、新旧 API Token 和 Dex 授权回调。验收期间**业务定时触发配置继续保持暂停**，验收通过后再按维护前记录恢复业务定时触发、入口和外部控制器。

验收通过后删除临时 Secret `zadig-aes-key-next`。切换完成后它与 `zadig-aes-key` 内容相同，只是多一份新密钥的明文副本，系统不会读取它。维护目录中的 `aes-next` 等文件按客户密钥管理要求保管或销毁。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" delete secret zadig-aes-key-next
```

`zadig-aes-key-previous` 保存的是旧密钥。删除之前，hub-server 仍接受用旧密钥加密的 Agent Token，旧密钥泄露时这段时间仍有风险。全部 Agent 完成升级并验证重连后，应尽快删除。

```bash
kubectl --context "$ZADIG_CONTEXT" -n "$ZADIG_NAMESPACE" delete secret zadig-aes-key-previous
```

JWT、API Token、MFA 恢复码将失效，用户需要重新登录/签发；本 Job 不删除其哈希。历史 pipeline `storage_uri` 只计数、不重写。

## 同步 Helm values 并更换 Dex 客户端密钥

AES 轮转并验收通过后，在有目标集群权限的运维终端运行同目录的 `sync-helm-values.sh`。脚本复用一次 Helm upgrade，同步显式保存的旧 AES key 并更换 Dex 客户端密钥；普通升级不调用它，UA 迁移 Job 和 admin 流程不变。

脚本会：
1. 备份 release 当前用户 values，并核对当前 AES Secret 等于维护目录中的 `aes-next`。
2. 仅将 values 中显式保存的本次旧 AES key 替换为新值；为空、未配置或已是新值时保持原样，遇到其他值则中止。
3. 从包含默认值的完整 Dex 客户端列表中更换 Zadig 客户端密钥，保留其他配置。
4. 使用指定的同一版本 Chart，带 `--no-hooks` 执行升级，跳过 UA 迁移、初始化和版本记录 hooks；等待服务就绪，并读回 Helm values 验证 AES 和 Dex 配置。

`--no-hooks` 只用于这次轮转收尾，不改变 Chart 的密钥解析逻辑：首次连接集群安装时，未设置密钥则自动生成；普通升级复用集群中已有的 AES 密钥，Dex 未显式设置时也复用已有密钥。本脚本显式更换 Dex 密钥，AES Secret 则已由前面的轮转步骤切换。普通安装和升级命令不增加此参数。它仍会更新整个 release 的资源，因此必须使用相同 Chart，不能借此步骤升级版本。

从仓库根目录执行，Chart 必须与当前部署一致；脚本依赖前面维护步骤生成的 `aes.before.json` 和 `aes-next`，不能在 AES 切换前执行。脚本独占创建维护目录下的 `helm-values-sync` 子目录保存备份和执行结果；已有该目录时直接退出，不覆盖上次记录或再次更换 Dex 密钥。失败或中断后先核对 release 实际状态；确需重试时，将原 `aes.before.json` 和同一个 `aes-next` 复制到新的维护目录后再执行，不要重新生成 AES key。

```bash
ZADIG_CHART='<当前已部署的 Chart，例如 koderover-chart/zadig>'
ZADIG_CHART_VERSION='<helm history 中当前的 Chart 版本>'
bash examples/encryption-key-rotation/sync-helm-values.sh \
  "$ROTATION_DIR" "$ZADIG_CONTEXT" "$ZADIG_NAMESPACE" "$ZADIG_RELEASE" \
  "$ZADIG_CHART" "$ZADIG_CHART_VERSION"
```

Dex 和 user 服务会自动重启，用户需要重新登录。完成后验收 Dex 授权回调和登录。维护目录中的 values 文件包含明文密钥，与维护目录一起按客户密钥管理要求保管或销毁。

## 限制与开发验证

旧密文使用无认证的 AES-CFB，S3 Secret 单独无法验证旧 key 是否正确。必须直接挂载当前 Secret，不能靠“解密无错误”判断 key 正确，也不能把工具当成可重复执行的迁移器。GitHub 按现有公共 API 客户端工作；GitLab/Gitee 使用记录中的实例地址。

本 PR 不提交测试文件，不提供依赖这些文件的测试命令。可构建 UA：

```bash
go build ./cmd/ua
```

本地隔离测试不能替代真实代码源验收。[Gitee 官方 PATCH 文档](https://gitee.com/api/v5/swagger)将事件字段标为可选，`push_events` 标注默认 `true`，未承诺省略字段会保留原值；当前 Secret 更新仅发送 URL/password，尚未完成真实 Gitee/Gitee EE 的事件开关前后对比，不应视为已通过验证。正式交付前须在专用测试仓库验证事件配置保持不变、Webhook 仍能触发，并完成客户代理/证书和完整维护窗口验收。
