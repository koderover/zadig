# PR 4667 需求文档：工作流 Payload 信息透传与动态通知人

## 背景

当前工作流支持在 Webhook 触发后执行任务，但通知对象主要依赖静态配置，无法直接从 Webhook payload 中提取代码 author、触发人或业务字段作为通知人。对于代码提交、合并请求、外部系统事件等场景，用户希望工作流执行完成后，IM 或邮件通知能够自动发送给 payload 中指定的人。

不同通知渠道对“人”的标识要求不同。例如飞书可能需要 user_id/open_id，钉钉需要手机号，邮件需要邮箱。如果要求用户在配置时额外声明 `identity_type`，前端和用户理解成本较高，也容易配置错误。因此本需求改为：用户只配置 payload 模板变量，后端根据变量字段后缀和通知渠道能力做确定性解析和转换。

## 目标

1. 工作流通知配置支持从 payload 中读取动态通知人。
2. 用户无需配置 `identity_type`，只填写字符串模板变量，例如 `{{.payload.user.email}}`。
3. 后端根据变量后缀识别身份类型，并按通知渠道转换成该渠道可发送的目标人标识。
4. 支持代码信息透传场景，任务执行阶段可以使用 Webhook payload 生成运行时变量，并用于通知人解析。
5. Webhook 触发的代码仓库上下文信息需要在任务生命周期内保留，并统一暴露为 `workflow.trigger.*` 运行时变量，包括分支、目标分支、PR、commit ID、commit message、committer、event 等，确保通知渲染、已有代码相关任务逻辑、重试和手动执行都可复用这些信息。
6. 对历史 PR 中间态数据保持读取兼容，避免已有任务重试或手动执行时因 schema 变化失败。

## 非目标

1. 不支持任意字符串作为动态通知人，例如 `someaccount`。
2. 不支持要求后端盲猜输入是邮箱、手机号、账号还是渠道 ID。
3. 不引入新的 `identity_type` 配置项。
4. 不保证每个渠道都支持所有身份类型，按渠道能力分别限制。
5. 不处理通知渠道外部账号体系本身不完整的问题，例如 Zadig 用户未填写手机号时无法发送钉钉通知。

## 用户配置方式

动态通知人字段统一为字符串数组：

```json
{
  "dynamic_recipients": [
    "{{.payload.user.email}}",
    "{{.payload.owner.mobile}}",
    "{{.payload.author.account}}"
  ]
}
```

模板变量必须是单个变量表达式，且最后一级字段名必须符合约定。

支持的字段后缀：

| 后缀 | 含义 |
| --- | --- |
| `email` | 邮箱 |
| `mobile` / `phone` | 手机号 |
| `account` | Zadig 用户账号 |
| `user_id` / `userid` | 渠道 user_id |
| `open_id` | 飞书 open_id |

## 代码信息透传范围

Webhook 触发 workflow v4 时，后端除了保留原始 payload 外，还需要保留供已有代码相关任务逻辑和任务链路重建复用的仓库上下文信息，至少包括：

| 信息 | 说明 |
| --- | --- |
| `branch` | 当前触发分支或匹配后的目标分支语义 |
| `tag` | tag 触发时的 tag 信息 |
| `pr` | PR / MR 编号 |
| `commit_id` | 触发本次任务的提交 ID |
| `commit_message` | 触发提交或 PR 标题对应的消息 |
| `committer` | 触发人/提交人标识 |
| `payload.*` | 从原始 Webhook JSON flatten 出来的叶子变量 |

当前 PR 对通知动态收件人、通知标题和通知内容渲染，明确保证可直接使用的是：

| 变量组 | 说明 |
| --- | --- |
| `payload.*` | 从原始 Webhook JSON flatten 出来的叶子变量 |
| `workflow.task.*` | 任务创建人、时间、URL 等运行时变量 |
| `workflow.params.*` | 工作流参数变量 |
| `workflow.trigger.*` | Webhook 统一触发变量，包括 branch、target_branch、pr、commit_id、commit_message、committer、event |
| `project` / `project.*` / `workflow.*` | 项目与工作流基础变量 |

其中 `workflow.trigger.*` 既服务于通知模板渲染，也保留代码仓库上下文在重试、手动执行、已有代码逻辑中的复用能力。

这些信息需要满足：

1. 首次 Webhook 触发任务时可用。
2. 任务重试时不丢失。
3. 手动执行阶段时不丢失。
4. 可同时服务于通知人解析（`payload.*`）和已有代码仓库相关任务逻辑。

## 渠道支持范围

| 通知渠道 | 支持的动态通知人类型 |
| --- | --- |
| 飞书群通知（应用群） | `email`、`mobile/phone`、`account`、`user_id` |
| 飞书个人通知 | `email`、`mobile/phone`、`account`、`user_id`、`open_id` |
| 飞书机器人 Webhook | `user_id` |
| 企业微信 Webhook | `user_id` |
| 钉钉 | `email`、`mobile/phone`、`account` |
| Microsoft Teams | `email`、`mobile/phone`、`account` |
| 邮件 | `email`、`mobile/phone`、`account` |

说明：飞书群通知不支持 `open_id`，飞书个人通知支持 `open_id`。

## 解析和转换规则

1. 用户创建任务时提交 `notify_inputs`，后端校验 `dynamic_recipients` 是否符合当前通知渠道支持范围。
2. 任务运行时，后端使用 runtime context 渲染模板变量。
3. 渲染结果为空或仍包含未解析模板时，该动态通知人跳过，不阻断通知。
4. 渲染结果非空时，后端按变量后缀确定身份类型，并转换为渠道需要的目标标识。
5. 转换结果去重后合并到原有静态通知人列表。

转换示例：

| 用户配置 | 钉钉通知 | 邮件通知 | 飞书通知 |
| --- | --- | --- | --- |
| `{{.payload.user.email}}` | 通过邮箱查 Zadig 用户，再取手机号 | 直接使用邮箱 | 优先用邮箱查飞书 user_id，查不到再通过 Zadig 用户手机号查 |
| `{{.payload.user.mobile}}` | 直接使用手机号 | 通过手机号查 Zadig 用户，再取邮箱 | 优先用手机号查飞书 user_id，查不到再通过 Zadig 用户邮箱查 |
| `{{.payload.user.account}}` | 通过账号查 Zadig 用户，再取手机号 | 通过账号查 Zadig 用户，再取邮箱 | 通过账号查 Zadig 用户，再用邮箱/手机号查飞书 user_id |

## 兼容性要求

1. 新接口和新持久化结构使用 `dynamic_recipients: []string`。
2. 兼容 PR 中间态旧结构：`[{ "value": "...", "identity_type": "..." }]`。
3. 旧结构读取时只保留 `value`，不继续使用 `identity_type`。
4. 如果旧结构中的 `value` 不是合法模板变量，运行时 fail fast，返回清晰错误。
5. Webhook 透传产生的 payload 和代码仓库上下文信息，在任务重试、手动执行阶段仍需可复用。

## 性能要求

1. 不做全类型盲猜，不对一个输入依次尝试 email、phone、account、user_id。
2. 只按字段后缀触发必要查询。
3. 单次通知解析内复用请求级缓存，避免重复查询同一账号、邮箱、手机号或飞书 user_id。
4. 为 `user.email` 和 `user.phone` 增加数据库索引，避免转换首次落地后出现全表扫描风险。

## 验收标准

1. 用户配置 `{{.payload.xxx.email}}` 后，邮件和 Teams 能收到邮箱通知。
2. 用户配置 `{{.payload.xxx.email}}` 后，钉钉可通过 Zadig 用户信息转换成手机号发送。
3. 用户配置 `{{.payload.xxx.email}}` 或 `{{.payload.xxx.mobile}}` 后，飞书可转换成 user_id 通知。
4. 用户配置不支持的后缀或渠道不支持的身份类型时，创建任务阶段返回明确错误。
5. 飞书群通知配置 `open_id` 时校验失败，飞书个人通知配置 `open_id` 时允许。
6. 已存在的 PR 中间态动态通知人数据可被读取为字符串数组。
7. MySQL 新库和升级场景都具备 `idx_email`、`idx_phone` 索引。
8. Webhook 触发任务后，`payload.*` 和 `workflow.trigger.*` 可在首次执行、重试、手动执行阶段继续用于通知变量渲染；代码仓库上下文信息可在这些阶段继续用于已有任务链路重建和代码相关逻辑。
