# PR 4667 API 文档：Payload 透传与动态通知人

## 1. 动态通知人配置

### 接口范围

动态通知人能力用于工作流任务运行时通知配置，主要影响以下接口：

| 场景 | 路由 |
| --- | --- |
| Zadig OpenAPI 创建自定义工作流任务 | `POST /api/aslan/workflow/openapi/custom/task` |
| Zadig 控制台创建工作流任务 | `POST /api/aslan/workflow/v4/workflowtask` |
| 工作流任务手动执行 / 重试 | 复用任务内已保存的通知配置和 runtime context |

说明：实际网关前缀以部署环境为准，表中展示的是 Aslan workflow router 下的路径语义。

### 请求字段

`notify_inputs` 中各通知配置新增或变更 `dynamic_recipients` 字段。

字段统一为字符串数组：

```json
{
  "notify_inputs": [
    {
      "id": 0,
      "type": "mail",
      "mail_notification_config": {
        "dynamic_recipients": [
          "{{.payload.user.email}}"
        ]
      }
    }
  ]
}
```

不再支持新写入以下结构：

```json
{
  "dynamic_recipients": [
    {
      "value": "{{.payload.user.email}}",
      "identity_type": "email"
    }
  ]
}
```

旧结构仅用于服务端读取兼容，不作为新 API 契约。

### `notify_inputs` 基本结构

```json
{
  "workflow_key": "workflow-demo",
  "project_key": "project-demo",
  "parameters": [],
  "inputs": [],
  "notify_inputs": [
    {
      "id": 0,
      "type": "mail",
      "enabled": true,
      "mail_notification_config": {
        "users": [],
        "user_ids": [],
        "dynamic_recipients": [
          "{{.payload.commits.0.author.email}}"
        ]
      }
    }
  ]
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | integer | 是 | 工作流通知配置下标，从 `0` 开始 |
| `type` | string | 是 | 通知类型，必须和工作流中第 `id` 个通知配置类型一致 |
| `enabled` | boolean | 否 | 运行时是否启用该通知；不传则保持原配置 |
| `*_notification_config.dynamic_recipients` | string[] | 否 | 动态通知人模板变量 |

### 支持的通知类型

| `type` | 配置字段 |
| --- | --- |
| `feishu` | `lark_hook_notification_config` |
| `feishu_app` | `lark_group_notification_config` |
| `feishu_person` | `lark_person_notification_config` |
| `wechat_work` | `wechat_notification_config` |
| `dingding` | `dingding_notification_config` |
| `msteams` | `msteams_notification_config` |
| `mail` | `mail_notification_config` |

## 2. 动态通知人变量规则

### 格式

每个动态通知人必须是单个模板变量：

```text
{{.payload.user.email}}
```

不支持：

```text
payload.user.email
user@example.com
{{.payload.user.email}} <{{.payload.user.name}}>
```

### 支持后缀

后端根据最后一级字段名识别身份类型。

| 后缀 | 识别类型 |
| --- | --- |
| `email` | 邮箱 |
| `mobile` / `phone` | 手机号 |
| `account` | Zadig 用户账号 |
| `user_id` / `userid` | 渠道 user_id |
| `open_id` | 飞书 open_id |

示例：

| 变量 | 识别结果 |
| --- | --- |
| `{{.payload.user.email}}` | `email` |
| `{{.payload.user.phone}}` | `mobile` |
| `{{.payload.user.account}}` | `account` |
| `{{.payload.user.user_id}}` | `user_id` |
| `{{.payload.user.open_id}}` | `open_id` |

`{{.payload.user.email_address}}` 不支持，因为最后一级字段名不是 `email`。

### 渠道支持矩阵

| 通知类型 | 支持的变量后缀 |
| --- | --- |
| `feishu` | `user_id` |
| `feishu_app` | `email`、`mobile/phone`、`account`、`user_id` |
| `feishu_person` | `email`、`mobile/phone`、`account`、`user_id`、`open_id` |
| `wechat_work` | `user_id` |
| `dingding` | `email`、`mobile/phone`、`account` |
| `msteams` | `email`、`mobile/phone`、`account` |
| `mail` | `email`、`mobile/phone`、`account` |

## 3. 不同渠道请求示例

### 邮件通知

```json
{
  "id": 0,
  "type": "mail",
  "mail_notification_config": {
    "dynamic_recipients": [
      "{{.payload.author.email}}",
      "{{.payload.reviewer.account}}"
    ]
  }
}
```

解析行为：

| 输入类型 | 行为 |
| --- | --- |
| `email` | 直接作为邮件接收人 |
| `mobile/phone` | 查 Zadig 用户，取邮箱 |
| `account` | 查 Zadig 用户，取邮箱 |

### 钉钉通知

```json
{
  "id": 1,
  "type": "dingding",
  "dingding_notification_config": {
    "dynamic_recipients": [
      "{{.payload.author.email}}",
      "{{.payload.owner.mobile}}"
    ]
  }
}
```

解析行为：

| 输入类型 | 行为 |
| --- | --- |
| `mobile/phone` | 直接作为 `at_mobiles` |
| `email` | 查 Zadig 用户，取手机号 |
| `account` | 查 Zadig 用户，取手机号 |

### 飞书群通知（自建应用）

```json
{
  "id": 2,
  "type": "feishu_app",
  "lark_group_notification_config": {
    "chat_id": "oc_xxx",
    "dynamic_recipients": [
      "{{.payload.author.email}}",
      "{{.payload.reviewer.user_id}}"
    ]
  }
}
```

解析行为：

| 输入类型 | 行为 |
| --- | --- |
| `user_id` | 直接作为飞书 user_id |
| `email` | 优先用邮箱查飞书 user_id；查不到则查 Zadig 用户手机号再查飞书 user_id |
| `mobile/phone` | 优先用手机号查飞书 user_id；查不到则查 Zadig 用户邮箱再查飞书 user_id |
| `account` | 查 Zadig 用户，再用邮箱/手机号查飞书 user_id |

`feishu_app` 不支持 `open_id`。

### 飞书个人通知

```json
{
  "id": 3,
  "type": "feishu_person",
  "lark_person_notification_config": {
    "dynamic_recipients": [
      "{{.payload.author.open_id}}",
      "{{.payload.reviewer.email}}"
    ]
  }
}
```

`feishu_person` 支持 `open_id` 和 `user_id`。

### 飞书机器人 / 企业微信机器人

```json
{
  "id": 4,
  "type": "feishu",
  "lark_hook_notification_config": {
    "dynamic_recipients": [
      "{{.payload.author.user_id}}"
    ]
  }
}
```

`feishu` 和 `wechat_work` 只支持 `user_id`，不做邮箱/手机号/账号转换。

## 4. Webhook Payload 透传变量

### 数据来源

GitHub、GitLab、Gitee、Gerrit 触发 workflow v4 时，服务端会保存原始 webhook request body 到任务的 hook payload 中：

```go
HookPayload.RawPayload
```

运行时变量渲染时，后端将 `RawPayload` 解析为 JSON，并 flatten 成 `payload.*` 变量。

同时，代码仓库上下文信息不会因为引入 payload 透传而丢失，运行时统一暴露以下 `workflow.trigger.*` 变量：

| 变量 | 说明 |
| --- | --- |
| `workflow.trigger.branch` | 当前触发分支语义 |
| `workflow.trigger.target_branch` | PR/MR 目标分支，或 push/tag 匹配后的目标分支语义 |
| `workflow.trigger.pr` | PR / MR 编号 |
| `workflow.trigger.commit_id` | 触发本次任务的 commit ID |
| `workflow.trigger.commit_message` | 触发提交消息或 PR 标题 |
| `workflow.trigger.committer` | 触发人 / 提交人 |
| `workflow.trigger.event` | 事件类型 |

这些信息用于已有 repo/runtime 变量链路，以及重试、手动执行等场景。

说明：当前通知 job 直接使用的渲染变量来自 `BuildWorkflowRuntimeVariableKVs`，明确包括 `payload.*`、`workflow.task.*`、`workflow.params.*`、`workflow.trigger.*` 以及 project/workflow 基础变量。

示例：

```text
{{.workflow.trigger.branch}}
{{.workflow.trigger.target_branch}}
{{.workflow.trigger.pr}}
{{.workflow.trigger.commit_id}}
{{.workflow.trigger.commit_message}}
{{.workflow.trigger.committer}}
{{.workflow.trigger.event}}
```

### Flatten 规则

| JSON 类型 | 变量生成规则 |
| --- | --- |
| object | 用字段名继续展开 |
| array | 用数字下标展开 |
| string/number/bool | 生成叶子变量 |
| null | 不生成变量 |

示例 payload：

```json
{
  "user": {
    "email": "dev@example.com",
    "mobile": "13800000000"
  },
  "commits": [
    {
      "author": {
        "email": "author@example.com"
      }
    }
  ]
}
```

可用变量：

```text
{{.payload.user.email}}
{{.payload.user.mobile}}
{{.payload.commits.0.author.email}}
```

### 存储说明

`payload.*` 变量不会持久化到 `GlobalContext`。原因是 raw payload 已经保存在 `HookPayload.RawPayload`，运行时按需解析即可，避免在 MongoDB 中重复存储大量 payload 展开字段。

代码仓库上下文信息则继续沿用已有 workflow/task 数据结构保存和重建，不依赖把所有 `payload.*` 展开后落库。

### 重试 / 手动执行行为

对于 webhook 触发的任务：

1. 首次执行时，`payload.*` 来源于 `HookPayload.RawPayload`。
2. 重试时，会重建 runtime context，继续使用同一份 hook payload 和已保存的代码仓库上下文。
3. 手动执行阶段时，也会重建 runtime context，确保动态通知人仍能基于 payload 变量渲染，并保留代码仓库上下文供已有任务链路复用。

因此，以下两类变量都应继续可用：

```text
{{.payload.user.email}}
{{.payload.commits.0.author.email}}
{{.workflow.trigger.target_branch}}
{{.workflow.trigger.commit_id}}
```

以及已有的代码仓库相关 runtime/repo 语义会继续保留在 workflow/task 链路中，供已有代码逻辑复用。

## 5. 用户搜索 API 扩展

### 接口

```text
POST /api/v1/users/search
```

### 请求体

```json
{
  "email": "dev@example.com"
}
```

或：

```json
{
  "phone": "13800000000"
}
```

或跨身份源账号查询：

```json
{
  "account": "dev01",
  "identity_type": "*"
}
```

### 查询优先级

当请求体同时包含多个条件时，后端按以下顺序选择一种查询：

1. `uids`
2. `email`
3. `phone`
4. `account`
5. 列表查询

### 响应体

```json
{
  "users": [
    {
      "uid": "user-uid",
      "name": "Dev User",
      "email": "dev@example.com",
      "phone": "13800000000",
      "identity_type": "system",
      "account": "dev01"
    }
  ],
  "totalCount": 1
}
```

说明：动态通知人解析内部会复用该用户查询能力，用于邮箱、手机号、账号之间的转换。

## 6. 错误行为

### 非单变量格式

请求：

```json
{
  "dynamic_recipients": ["dev@example.com"]
}
```

结果：创建任务失败，错误信息类似：

```text
dynamic recipient must be a single template variable, got dev@example.com
```

### 不支持的字段后缀

请求：

```json
{
  "dynamic_recipients": ["{{.payload.user.email_address}}"]
}
```

结果：创建任务失败，错误信息会提示允许的后缀：

```text
only email/mobile(phone)/account/user_id(userid)/open_id are allowed
```

### 渠道不支持该类型

请求：

```json
{
  "type": "feishu_app",
  "lark_group_notification_config": {
    "dynamic_recipients": ["{{.payload.user.open_id}}"]
  }
}
```

结果：创建任务失败，因为飞书群通知不支持 `open_id`。

## 7. 兼容性

服务端读取 workflow/task 中的旧中间态数据时，兼容以下结构：

```json
{
  "dynamic_recipients": [
    {
      "value": "{{.payload.user.email}}",
      "identity_type": "email"
    }
  ]
}
```

兼容行为：

1. 只读取 `value`。
2. 丢弃 `identity_type`。
3. 后续仍按新规则校验和解析。
