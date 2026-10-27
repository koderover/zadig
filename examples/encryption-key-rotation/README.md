# UA 密钥轮换验证

这是独立维护 Job，不属于 Chart，也不是 Helm Hook。只执行存量密文迁移和远端 Webhook Secret 更新，不切换运行中的 AES/Dex Secret。

## 前提

- 已先升级到兼容的业务镜像和 Chart，仍使用原 AES/Dex Secret。
- 关闭业务入口、工作流/定时触发/Webhook 消费，结束运行任务，停止 user、aslan、hub-server 等密文读写服务。
- 停写后完成 MongoDB、MySQL 用户库和 Dex 库、当前 AES Secret、Helm values、Dex Secret、Webhook 清单和 Agent YAML 的最终备份。
- 本工具只支持 MySQL。代码源 OAuth Token 须在维护前确认有效；dry-run 不刷新 Token、不写数据库或 Redis。
- 使用已发布且支持节点架构的 UA 镜像。客户节点只需正常拉取镜像，不需要 Docker；本地构建/推送工具不属于客户节点依赖。

## 执行

以下命令使用当前 kubectl context。先确认目标集群和安装命名空间；命令中的 `<安装命名空间>`、镜像地址需要替换。

```bash
kubectl config current-context
helm list -A
```

生成 32 字节的新 AES key（文件无换行），创建临时 Secret。文件按客户密钥管理要求保管，不提交 Git。

```bash
umask 077
openssl rand -hex 16 | tr -d '\n' > aes-next
kubectl -n <安装命名空间> create secret generic zadig-aes-key-next --from-file=aesKey=./aes-next
```

修改 `job.yaml` 中的镜像为本次发布的 UA 完整地址。私有仓库在 Pod spec 添加已有的 `imagePullSecrets`。Job 从现有 `user-config` 读取 MongoDB、MySQL、Redis 配置（包括 `REDIS_USER_TOKEN_DB`）；代码源代理按数据库中现有配置读取。核对该 ConfigMap 与当前部署一致；使用自定义证书的代码源须挂载客户 CA 并设置 `SSL_CERT_FILE`。

```bash
kubectl -n <安装命名空间> create -f job.yaml
kubectl -n <安装命名空间> logs -f job/zadig-key-rotation-dry-run
kubectl -n <安装命名空间> get job zadig-key-rotation-dry-run
```

成功报告应为 `phase: validated`，各类 `failed` 为 0；dry-run 的 `succeeded` 为 0，解密通过数量见 `validated`。检查 `manual_webhooks`，记录需要人工更新的仓库。

确认 dry-run 成功且仍在停写状态后，复制 `job.yaml` 为 `job-apply.yaml`，将 Job 名改为 `zadig-key-rotation-apply`，删除 `--dry-run` 这一行，其余配置保持一致。

```bash
kubectl -n <安装命名空间> create -f job-apply.yaml
kubectl -n <安装命名空间> logs -f job/zadig-key-rotation-apply
kubectl -n <安装命名空间> get job zadig-key-rotation-apply
```

正式成功报告为 `phase: completed`，所有 `failed` 为 0。S3、Agent、MFA 和托管 Webhook 的 `succeeded + skipped` 应等于 `total`。按报告手工更新无 Hook ID 的 Webhook，其 Secret 为新 AES key 的 SHA-256 十六进制值。

**任何写入失败后保持维护状态，先恢复最终 MongoDB/MySQL 备份，再修复并重跑完整 Job。禁止直接在已迁移或部分迁移的数据上重跑；Job 不会自动重试。** 已更新的远端 Webhook 可重复设置相同新 Secret；放弃轮换时需按备份恢复旧 Webhook Secret。

## 迁移后的步骤

继续执行客户方案中的 AES/Dex 切换：将旧 key 保存在 `zadig-aes-key-previous`，将当前 `zadig-aes-key` 更新为新 key；执行第二次 Helm upgrade，显式传入 `--set-string global.encryption.key=` 并更新 Dex Client Secret。确认新 key 指纹一致，重启并验证 user、aslan、hub-server、Dex、cron，验收 Agent 重连、MFA、对象存储、Webhook、API Token 后恢复业务。

JWT、API Token、MFA 恢复码将失效，用户需要重新登录/签发；本 Job 不删除其哈希。历史 pipeline `storage_uri` 只计数、不重写。全部 Agent 完成升级并验证后，按方案删除过渡 Secret。

## 限制与开发验证

旧密文使用无认证的 AES-CFB，S3 Secret 单独无法验证旧 key 是否正确。必须直接挂载当前 Secret，不能靠“解密无错误”判断 key 正确，也不能把工具当成可重复执行的迁移器。GitHub 按现有公共 API 客户端工作；GitLab/Gitee 使用记录中的实例地址。

集成测试使用独立、可销毁的 MongoDB/MySQL/Redis，**不要将测试连接变量指向业务库**（测试会建删库并清空指定 Redis DB）：

```bash
ROTATION_TEST_MONGO='<隔离 MongoDB URI>' \
ROTATION_TEST_MYSQL='<隔离 MySQL 管理 DSN，以 / 结尾>' \
ROTATION_TEST_REDIS='<隔离 Redis host:port>' \
go test ./pkg/cli/upgradeassistant/internal/rotation -count=1 -v
go build ./cmd/ua
```

数据库测试覆盖 dry-run 零写入、预检失败、重加密和写入失败；代码源测试使用本地 HTTP 服务验证请求及脱敏，真实代码源权限、客户代理/证书和完整维护窗口验收仍须在测试环境执行。
