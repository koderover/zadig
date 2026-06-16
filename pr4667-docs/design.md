# PR 4667 设计文档：工作流动态通知人解析与渠道身份转换

## 设计概述

本 PR 将通知配置中的动态通知人从结构体数组改为字符串数组，用户只需要配置 payload 模板变量：

```json
{
  "dynamic_recipients": ["{{.payload.user.email}}"]
}
```

后端通过变量最后一级字段名推断身份类型，例如 `email`、`mobile`、`account`、`user_id`、`open_id`。解析过程不做盲猜，而是在创建任务提交 `notify_inputs` 时做格式和渠道能力校验，在任务运行时按确定类型完成渠道转换。

## 接口设计

### 输入结构

所有通知配置中的 `dynamic_recipients` 统一为 `[]string`。

示例：

```json
{
  "mail_notification_config": {
    "dynamic_recipients": [
      "{{.payload.commit.author.email}}"
    ]
  }
}
```

旧结构已废弃：

```json
{
  "dynamic_recipients": [
    {
      "value": "{{.payload.commit.author.email}}",
      "identity_type": "email"
    }
  ]
}
```

### 校验规则

`dynamic_recipients` 中每一项必须满足：

1. 是单个模板变量，格式为 `{{.xxx.xxx}}`。
2. 最后一级字段名在允许列表中。
3. 字段后缀对应的身份类型被当前通知渠道支持。

允许后缀：

| 后缀 | 内部类型 |
| --- | --- |
| `email` | `email` |
| `mobile` / `phone` | `mobile` |
| `account` | `account` |
| `user_id` / `userid` | `user_id` |
| `open_id` | `open_id` |

## 渠道能力矩阵

| 渠道 | 内部通知类型 | 支持类型 | 输出目标 |
| --- | --- | --- | --- |
| 飞书机器人 Webhook | `feishu` | `user_id` | `at_users` |
| 飞书群通知 | `feishu_app` | `email`、`mobile`、`account`、`user_id` | `AtUsers`，ID type 为 user_id |
| 飞书个人通知 | `feishu_person` | `email`、`mobile`、`account`、`user_id`、`open_id` | `TargetUsers`，支持 user_id/open_id |
| 企业微信 Webhook | `wechat_work` | `user_id` | `at_users` |
| 钉钉 | `dingding` | `email`、`mobile`、`account` | `at_mobiles` |
| Microsoft Teams | `msteams` | `email`、`mobile`、`account` | `at_emails` |
| 邮件 | `mail` | `email`、`mobile`、`account` | `target_users` |

飞书群通知不允许 `open_id`，因为运行路径按 user_id 发送；飞书个人通知允许 `open_id`。

## 数据流

1. 前端或 API 提交通知配置。
2. `updateNotifyCtls` 裁剪空字符串，并调用 `ValidateDynamicRecipientsForNotifyType` 做边界校验。
3. 通知配置持久化到 workflow/task 的通知配置中。
4. Webhook 触发 workflow v4 时，原始 request body 保存到 `HookPayload.RawPayload`，同时仓库上下文中的 `branch`、`target_branch`、`pr`、`commit_id`、`commit_message`、`committer`、`event` 等信息保留到 `HookPayload` 中。
5. 工作流任务运行时构造 runtime context；通知渲染直接使用 `payload.*`、`workflow.task.*`、`workflow.params.*`、`workflow.trigger.*` 以及 project/workflow 基础变量。
5. `NotificationJobCtl.resolveDynamicRecipients` 按通知渠道调用对应 resolver。
6. resolver 渲染模板变量，得到实际值。
7. resolver 按变量后缀和渠道能力转换目标人。
8. 转换结果去重后合并到静态通知人列表。
9. 后续发送逻辑复用原有通知发送链路。

## 代码信息透传设计

### 透传内容

本次 PR 中“代码信息透传”不是只透传原始 payload，而是分成两层：

1. 原始 webhook body 以 `HookPayload.RawPayload` 保存。
2. 代码仓库上下文信息继续保留在 workflow/task 运行链路中，包括：
   - `branch`
   - `tag`
   - `pr`
   - `commit_id`
   - `commit_message`
   - `committer`

这些信息一部分来自 webhook matcher 对仓库对象的补全，一部分来自 `HookPayload` 字段持久化；运行时统一通过 `workflow.trigger.*` 暴露给通知模板和工作流变量引用。

### 统一运行时变量

当前 PR 统一暴露以下触发变量：

| 变量 | 说明 |
| --- | --- |
| `workflow.trigger.branch` | 触发分支 |
| `workflow.trigger.target_branch` | 目标分支，PR/MR 场景为目标分支，push/tag 场景为匹配分支语义 |
| `workflow.trigger.pr` | PR / MR 编号 |
| `workflow.trigger.commit_id` | 触发本次任务的 commit 标识 |
| `workflow.trigger.commit_message` | 提交消息或 PR 标题 |
| `workflow.trigger.committer` | 提交人 / 触发人 |
| `workflow.trigger.event` | 统一事件标识；GitHub/GitLab/Gitee 为 `pr` / `push` / `tag`，Gerrit 为原始事件类型 |

### payload 变量生成

运行时渲染阶段，后端将 `HookPayload.RawPayload` 解析为 JSON，并 flatten 为 `payload.*` 变量，例如：

```text
{{.payload.user.email}}
{{.payload.commits.0.author.email}}
```

这部分变量是运行时按需生成的，不直接展开后持久化进 MongoDB。

### 代码仓库上下文保留

对于已有 repo 相关运行时语义，设计上继续保留原有行为，不让 payload 透传破坏已有代码任务链路。当前保留的典型信息包括：

| 信息 | 用途 |
| --- | --- |
| `workflow.trigger.branch` | 构建、部署等已有代码任务中的分支语义 |
| `workflow.trigger.target_branch` | PR / MR 目标分支，或匹配后的触发分支语义 |
| `workflow.trigger.pr` | PR / MR 场景复用 |
| `workflow.trigger.commit_id` | 定位具体提交 |
| `workflow.trigger.commit_message` | 从提交消息中继续提取环境变量片段 |
| `workflow.trigger.committer` | 代码触发链路中的触发人标识 |
| `workflow.trigger.event` | 按渠道/代码源区分事件类型 |

说明：这组变量通过 `BuildWorkflowRuntimeVariableKVs`、`getWorkflowDefaultParams` 和变量引用列表统一暴露，因此通知 job 渲染、默认参数渲染、变量选择面板保持一致。

### 重试和手动执行

本次设计要求代码信息透传不仅在首次 webhook 触发时可用，还要在以下场景继续可用：

1. `RetryWorkflowTaskV4`
2. `ManualExecWorkflowTaskV4`

实现方式是重建任务 runtime context，并复用 workflow args、hook payload 和已有 global/job output 信息，而不是只依赖首次执行现场内存数据。

## 转换策略

### 邮件 / Teams

目标需要邮箱。

| 输入类型 | 转换 |
| --- | --- |
| `email` | 直接作为邮箱 |
| `mobile` | 按手机号查询 Zadig 用户，取用户邮箱 |
| `account` | 按账号查询 Zadig 用户，取用户邮箱 |

### 钉钉

目标需要手机号。

| 输入类型 | 转换 |
| --- | --- |
| `mobile` | 直接作为手机号 |
| `email` | 按邮箱查询 Zadig 用户，取用户手机号 |
| `account` | 按账号查询 Zadig 用户，取用户手机号 |

### 飞书群 / 飞书个人

目标主要使用飞书 user_id。

| 输入类型 | 转换 |
| --- | --- |
| `user_id` | 直接作为飞书 user_id |
| `open_id` | 仅飞书个人通知允许，直接作为 open_id |
| `email` | 先用邮箱查询飞书 user_id；查不到时按邮箱查 Zadig 用户，再用手机号查询飞书 user_id |
| `mobile` | 先用手机号查询飞书 user_id；查不到时按手机号查 Zadig 用户，再用邮箱查询飞书 user_id |
| `account` | 按账号查 Zadig 用户，再依次用邮箱、手机号查询飞书 user_id |

### 飞书机器人 Webhook / 企业微信 Webhook

只支持 `user_id`，不做用户信息转换。

## 用户查询扩展

`/users/search` 新增支持按 `email` 和 `phone` 查询用户。

查询优先级：

1. `uids`
2. `email`
3. `phone`
4. `account`
5. 常规列表查询

账号查询支持 `identity_type="*"`，用于跨身份源查找同一账号。动态通知人解析使用该能力，避免调用方必须知道用户来自本地、LDAP、OAuth 或其它身份源。

## 存储与兼容

### 新结构

持久化字段为：

```go
type DynamicRecipients []string
```

各通知配置仍使用字段名 `dynamic_recipients`。

Webhook 透传部分沿用已有 `HookPayload` 结构，其中 `RawPayload` 用于保存原始 Webhook JSON，请求运行时再按需展开为 `payload.*` 变量。

### 旧中间结构兼容

为了兼容 PR 中间态已经落库的数据，`DynamicRecipients` 支持反序列化：

```json
[
  {
    "value": "{{.payload.user.email}}",
    "identity_type": "email"
  }
]
```

兼容策略：

1. JSON/BSON 读取时优先按 `[]string` 解码。
2. 解码失败后按旧结构解码。
3. 旧结构只保留 `value`。
4. 不继续使用 `identity_type`，避免废弃 schema 变成新契约。

## 性能设计

### 避免长查询链

本设计不对同一个输入做 email、phone、account、user_id 的盲猜。变量后缀决定身份类型，因此查询链是确定的。

### 请求级缓存

单次通知解析内缓存：

1. account -> Zadig users
2. email -> Zadig users
3. phone -> Zadig users
4. appID + queryType + value -> 飞书 user_id
5. 飞书 user_id miss 结果
6. appID -> 飞书 client

这样同一个通知配置中多处引用相同用户信息时，不会重复打用户服务或飞书接口。

### 数据库索引

新增索引：

| 字段 | 索引名 |
| --- | --- |
| `user.email` | `idx_email` |
| `user.phone` | `idx_phone` |

覆盖范围：

1. MySQL 初始化 SQL。
2. 达梦初始化 SQL。
3. GORM model tag。
4. 4.3.0 upgradeassistant 迁移，用于已有线上库补索引。

## 错误处理

1. 配置格式不合法时，在创建任务并提交 `notify_inputs` 阶段返回错误。
2. 渠道不支持某类型时，在创建任务并提交 `notify_inputs` 阶段返回错误。
3. 模板运行时渲染为空或未解析完成时跳过该动态收件人。
4. 外部查询报错时返回错误，避免静默丢通知。
5. 查询不到用户或用户缺少必要字段时跳过该用户，不阻断其它通知人。

## 取舍说明

### 为什么去掉 `identity_type`

保留 `identity_type` 会要求用户同时配置“变量值”和“变量含义”，例如：

```json
{
  "value": "{{.payload.user.email}}",
  "identity_type": "email"
}
```

这增加了配置负担，也容易出现 `value` 是邮箱但 `identity_type` 写成手机号的冲突。当前设计改为通过字段后缀表达语义，用户只需要配置一个字符串变量。

### 为什么不支持任意变量名

例如 `{{.payload.contact.email_address}}` 无法通过后缀识别身份类型。为了避免后端猜测和长查询链，当前要求用户使用约定后缀，例如 `email`、`mobile`、`account`。

### 为什么旧结构只兼容读取 `value`

`identity_type` 是 PR 中间态字段，不是最终产品契约。继续使用它会把临时 schema 固化为正式行为，因此只做读取兼容，不在运行时继续依赖它。

## 风险与缓解

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 用户 payload 字段命名不符合后缀约定 | 配置校验失败或运行时报错 | 错误信息列出允许后缀，文档明确命名规范 |
| 代码信息只在首次 webhook 触发时可用，重试/手动执行丢失 | 代码相关任务链路行为不一致 | 重建 runtime context 并复用 hook payload / workflow args |
| Zadig 用户缺少邮箱或手机号 | 无法转换到部分渠道 | 跳过该用户，不影响其它可解析用户 |
| 飞书外部接口查询失败 | 当前通知任务返回错误 | 复用原有错误链路，保留可观测错误 |
| 已有库缺少 email/phone 索引 | 查询可能全表扫 | 初始化 SQL 和 upgradeassistant 迁移补索引 |
| PR 中间态旧数据 schema 不一致 | 老任务重试可能读取失败 | `DynamicRecipients` 自定义 JSON/BSON 解码兼容 |

## 验证建议

1. 通过 Webhook payload 提供 `payload.user.email`，配置邮件动态通知人，确认邮件目标包含该邮箱。
2. 通过 GitHub/GitLab/Gitee/Gerrit webhook 触发任务，确认 `payload.*` 和 `workflow.trigger.*` 可用于通知变量渲染，且这些代码信息在运行时仍可被已有代码逻辑消费。
2. 通过同一邮箱查到 Zadig 用户手机号，配置钉钉动态通知人，确认 `at_mobiles` 被补充。
3. 配置飞书个人通知 `{{.payload.user.open_id}}`，确认允许保存并可通知。
4. 配置飞书群通知 `{{.payload.user.open_id}}`，确认创建任务时报错。
5. 模拟旧结构 `{value, identity_type}` 存量数据，确认可反序列化并继续运行。
6. 检查新库和升级库均存在 `idx_email`、`idx_phone`。
7. 在重试和手动执行阶段，再次验证 `payload.*` 仍可用于通知变量渲染，代码仓库上下文仍可用于已有任务链路重建。
