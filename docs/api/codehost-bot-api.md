# 代码源机器人身份

为代码源配置机器人身份后，AI 代码审查的总结评论和行内评论将以机器人的名义发布到 Pull Request / Merge Request 中。

本文用于前后端对接，描述代码源机器人身份的接口约定。机器人配置放在现有代码源表单的「高级配置」中，通过原代码源接口的 `bot` 字段统一查询和保存。

**适用范围**

- 仅影响 AI 代码审查的总结评论和行内评论。代码拉取、Webhook、Git Check、普通工作流通知等继续使用现有逻辑
- 支持的代码源类型及优先级：P0 `gitlab`，P1 `github`。GitHub Enterprise 不在本次范围内
- 仅支持系统级代码源，创建和修改配置需为系统管理员；查询权限沿用现有代码源接口
- 机器人身份为可选配置，不配置时保持现有行为

**评论身份规则**

总结评论和行内评论使用同一套规则：

| 代码源类型 | 未配置机器人身份 | 已配置机器人身份 |
| ---------- | ---------------- | ---------------- |
| `gitlab` | 使用代码源自身的授权凭证（现有行为） | 使用机器人 Token |
| `github` | 系统中存在全局 GitHub App 时使用全局 App，否则使用代码源自身的授权凭证（现有行为） | 使用代码源配置的 GitHub App，优先于全局 App |

删除机器人身份配置后，恢复为「未配置」时的行为。

已配置机器人身份时，若机器人发布评论失败（如 GitHub App 未安装到目标仓库、GitLab Token 已过期或无项目权限），会在工作流日志中记录明确的错误信息，并沿用现有的评论失败处理方式，不会改用其他身份重新发布。

**实现约束**

- 机器人凭证校验和评论客户端沿用代码源的地址、代理（`enable_proxy`）和 TLS（`disable_ssl`）设置；GitHub 查询 Installation 的既有网络限制见「GitHub 客户端」
- GitLab 机器人 Token 独立存储，不参与代码源原有的 OAuth Token 刷新逻辑
- 私钥和 Token 作为敏感信息存储，任何 HTTP 接口均不返回其内容；评论发布通过内部 Go 调用读取凭证

## 前端接入流程

1. 在系统级 GitLab、GitHub 代码源的「高级配置」中增加可选的「AI 代码审查机器人身份」配置项，与代码源共用保存按钮。
2. 编辑时通过原代码源详情或列表接口读取 `bot`。`bot.configured` 为 `true` 时展示已配置状态和机器人身份信息；Token、私钥输入框保持为空，不展示占位密文。
3. 新增代码源时，将机器人配置一并放入原 POST 请求。编辑时，将变更的机器人配置合并到原 PATCH 请求，原代码源表单字段继续按现有逻辑提交。
4. 已配置的凭证未修改时不传该凭证字段；机器人配置整体未修改时不传 `bot`。用户明确移除已保存的机器人配置时传 `bot: null`。
5. 保存成功后使用响应中的 `bot` 更新表单状态，并清空凭证输入框。保存失败时展示 `description`，本次代码源和机器人配置均不生效。取消编辑不发送保存请求。

无需新增机器人配置页面、独立保存按钮或独立测试连接请求。保存时由后端校验机器人凭证。

## 查询代码源中的机器人配置

**请求**

```
GET /api/v1/codehosts/:id?encryptedKey=<encryptedKey>
```

系统级代码源列表 `GET /api/v1/codehosts?encryptedKey=...` 中的每一项也返回相同的 `bot` 结构，原有筛选参数和返回数组结构保持不变。

**路径参数说明**

| 参数名 | 类型 | 描述 | 是否必须 | 默认值 |
| ------ | ---- | ---- | -------- | ------ |
| `id` | int | 代码源 ID | 是 | 无 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 是否必须 | 默认值 |
| ------ | ---- | ---- | -------- | ------ |
| `encryptedKey` | string | 沿用现有代码源查询传参方式，不因新增机器人配置而取消 | 是 | 无 |
| `ignoreDelete` | bool | 仅详情接口使用，是否查询已删除记录；编辑页面使用默认值 | 否 | `false` |

**成功返回说明**

原有代码源字段和加密处理保持不变，新增 `bot` 字段。仅系统级 `gitlab`、`github` 代码源返回该字段，其他代码源省略。机器人凭证即使加密也不返回。

`bot` 字段说明：

| 参数名 | 说明 | 类型 |
| ------ | ---- | ---- |
| `configured` | 是否已配置机器人身份 | bool |
| `gitlab` | GitLab 机器人配置，仅 `type` 为 `gitlab` 且已配置时返回，字段说明见下表 | object |
| `github` | GitHub App 配置，仅 `type` 为 `github` 且已配置时返回，字段说明见下表 | object |
| `updated_by` | 最近一次修改人，未配置时不返回 | string |
| `updated_at` | 最近一次修改时间，Unix 时间戳格式，未配置时不返回 | int |

`bot.gitlab` 字段说明：

| 参数名 | 说明 | 类型 |
| ------ | ---- | ---- |
| `token_configured` | 是否已配置 Token | bool |
| `bot_username` | Token 对应的用户名，保存配置时自动获取 | string |

`bot.github` 字段说明：

| 参数名 | 说明 | 类型 |
| ------ | ---- | ---- |
| `app_id` | GitHub App ID | int |
| `private_key_configured` | 是否已配置私钥 | bool |
| `bot_name` | 评论作者显示名，格式为 `<App slug>[bot]`，保存配置时自动获取 | string |

**成功返回示例**

以下仅展示代码源响应中的相关字段；实际响应保留原有字段，不增加 `data` 等外层包装。

GitLab 代码源：

```json
{
    "id": 5,
    "type": "gitlab",
    "bot": {
        "configured": true,
        "gitlab": {
            "token_configured": true,
            "bot_username": "group_12_bot_3f2a8c1d"
        },
        "updated_by": "admin",
        "updated_at": 1792483200
    }
}
```

GitHub 代码源：

```json
{
    "id": 3,
    "type": "github",
    "bot": {
        "configured": true,
        "github": {
            "app_id": 1023456,
            "private_key_configured": true,
            "bot_name": "zadig-ai-review[bot]"
        },
        "updated_by": "admin",
        "updated_at": 1792483200
    }
}
```

未配置机器人身份时：

```json
{
    "id": 5,
    "type": "gitlab",
    "bot": {
        "configured": false
    }
}
```

未配置时 `bot` 仅返回 `configured: false`；`bot.gitlab`、`bot.github`、`bot.updated_by` 和 `bot.updated_at` 均省略。所属代码源 ID 和类型复用原响应的 `id`、`type`，不在 `bot` 中重复返回。

## 随代码源创建或保存机器人配置

复用现有代码源创建、更新接口，在原请求体中增加可选的 `bot` 字段。

**请求**

```
POST /api/v1/codehosts
PATCH /api/v1/codehosts/:id
```

POST 用于创建代码源并可同时配置机器人；PATCH 用于保存现有代码源及机器人配置。权限校验、原代码源字段和授权流程沿用现有接口。

**原代码源字段仍需按现有表单逻辑提交。现有 PATCH 不支持对所有字段按需更新，不能只提交下文的 `bot` 示例，否则可能覆盖原代码源字段。** 本文的部分更新规则仅针对新增的 `bot` 字段及其凭证。

**路径参数说明**

| 参数名 | 类型 | 描述 | 是否必须 | 默认值 |
| ------ | ---- | ---- | -------- | ------ |
| `id` | int | 代码源 ID，仅 PATCH 使用 | 是 | 无 |

**body 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| ------ | ---- | ---- | ---- |
| `bot` | 机器人配置对象；不传表示不修改，`null` 表示清除，具体规则见下表 | object / null | 否 |

`bot` 的提交规则：

| 请求中 `bot` 的值 | POST 创建 | PATCH 保存 |
| ----------------- | --------- | ---------- |
| 不传 | 不配置机器人 | 保留原机器人配置，不额外校验机器人凭证 |
| `null` | 不配置机器人 | 清除已保存的机器人配置，不校验旧机器人凭证 |
| 配置对象 | 校验完整凭证后创建 | 合并旧凭证并校验后保存 |
| `{}` | 参数错误 | 参数错误，不表示清除 |

`bot` 对象只传与代码源类型对应的一项，不能同时传 `gitlab` 和 `github`：

| 参数名 | 说明 | 类型 | 必填 |
| ------ | ---- | ---- | ---- |
| `gitlab` | GitLab 机器人配置，字段说明见下表 | object | 配置 GitLab 机器人时必填 |
| `github` | GitHub App 配置，字段说明见下表 | object | 配置 GitHub 机器人时必填 |

`bot.gitlab` 参数说明：

| 参数名 | 说明 | 类型 | 必填 |
| ------ | ---- | ---- | ---- |
| `token` | 机器人用户的 Access Token，scope 需包含 `api`，角色至少为 Reporter | string | 首次配置时必填；更新时不传或传空字符串表示沿用已保存的 Token |

`bot.github` 参数说明：

| 参数名 | 说明 | 类型 | 必填 |
| ------ | ---- | ---- | ---- |
| `app_id` | 正整数，在 GitHub App 设置页的 About 区域查看 | int | 首次配置时必填；更新时不传表示沿用已保存的 App ID |
| `private_key` | GitHub App 私钥，即下载的 `.pem` 文件完整内容 | string | 首次配置或 `app_id` 变更时必填；更新时不传或传空字符串表示沿用已保存的私钥 |

`configured`、`token_configured`、`private_key_configured`、`bot_username`、`bot_name`、`updated_by`、`updated_at` 均为响应字段，前端不要随保存请求回传。界面的配置项开关不对应额外的 `enabled` 字段；是否使用机器人由保存后是否存在完整配置决定。

**保存规则**

- 同一次请求中的代码源字段和机器人配置统一保存：先完成全部校验，再写入数据库。校验失败时不创建代码源，也不修改已有代码源或机器人配置，不出现一部分保存成功的情况
- 提交 `bot` 对象时，使用最终生效的凭证（新传入的值，或沿用的已保存值）校验有效性；地址、代理和 TLS 设置使用本次代码源保存后将生效的值
- GitLab：使用 Token 请求当前用户信息，成功则校验通过，并记录 `bot_username`
- GitHub：使用 App ID 和私钥请求 App 信息，成功则校验通过，并记录 `bot_name`
- 保存时不校验 App 是否已安装到具体仓库，也不校验机器人对具体项目的权限

**body 参数示例**

以下均为需要合并到原代码源完整表单请求中的 `bot` 字段片段，不能直接作为完整 PATCH 请求提交。

GitLab 首次配置或更换 Token：

```json
{
    "bot": {
        "gitlab": {
            "token": "glpat-xxxxxxxxxxxxxxxxxxxx"
        }
    }
}
```

GitHub 首次配置或更换 App（App ID 变更时需同时传入新私钥）：

```json
{
    "bot": {
        "github": {
            "app_id": 1023456,
            "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n"
        }
    }
}
```

GitHub 仅轮换私钥（App ID 沿用旧值）：

```json
{
    "bot": {
        "github": {
            "private_key": "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n"
        }
    }
}
```

清除已保存的机器人配置：

```json
{
    "bot": null
}
```

清除后，后续 AI 代码审查评论恢复为未配置时的身份规则。尚未配置时重复提交 `bot: null` 也成功；其他代码源字段仍正常保存。不要调用代码源 DELETE 接口，该接口会删除整个代码源。

**返回**

创建、保存均返回原代码源对象，并增加与查询相同的 `bot` 响应结构。清除配置后返回 `bot: {"configured": false}`。不返回机器人 Token 或私钥。

## 错误说明

**错误返回结构**

| 参数名 | 说明 | 类型 |
| ------ | ---- | ---- |
| `type` | 固定为 `error` | string |
| `code` | Zadig 业务错误码，部分标准错误码与 HTTP 状态码一致 | int |
| `message` | 错误类型，如 `Bad Request`、`Request Not Found` | string |
| `description` | 错误详情 | string |
| `extra` | 附加信息，通常为空 | object |

**错误返回示例**

```json
{
    "type": "error",
    "code": 400,
    "message": "Bad Request",
    "description": "GitLab Token 校验失败：401 Unauthorized",
    "extra": {}
}
```

**错误场景**

| 场景 | HTTP 状态码 | `description` 示例 |
| ---- | ----------- | ------------------ |
| 代码源不存在或已删除 | 404 | 代码源 5 不存在 |
| 不支持的代码源类型提交机器人配置 | 400 | 代码源类型 gitee 不支持配置机器人身份 |
| 非系统级代码源配置机器人身份 | 400 | 仅系统级代码源支持配置机器人身份 |
| 参数与代码源类型不匹配 | 400 | GitLab 代码源需传入 gitlab 配置 |
| `bot` 为空对象 | 400 | GitLab 代码源需传入 gitlab 配置 |
| 首次配置缺少凭证 | 400 | 首次配置需提供 token |
| `app_id` 不是正整数 | 400 | app_id 必须为正整数 |
| `app_id` 变更但未提供私钥 | 400 | 变更 app_id 时需提供对应的 private_key |
| 私钥格式错误 | 400 | private_key 不是合法的 PEM 格式私钥 |
| 凭证校验失败 | 400 | GitLab Token 校验失败：401 Unauthorized |
| 当前用户不是系统管理员 | 403 | 无返回体 |

代码源查询不到时，接口必须将底层的“未找到”错误转换为上述 404 响应，不应直接返回数据库错误。

本节列出机器人配置相关的错误约定；现有代码源接口的鉴权、`encryptedKey` 缺失、别名重复等错误沿用原有处理。原代码源校验或机器人校验失败，均不写入本次保存的任何字段。

## 内部实现方案

本节说明后端实现方案。前端对接以上述接口和字段约定为准。接口沿用代码源模块现有的 `Handler -> Service -> Repository` 分层，评论发布沿用 AI 审查报告的处理流程。

### 配置存储

在现有 `code_host` 文档中增加可选的 `bot` 子文档，每个代码源保存一份机器人配置。未配置时不存储该字段，无需迁移已有数据。

| 存储字段 | 类型 | 说明 |
| -------- | ---- | ---- |
| `bot.gitlab.token` | string | GitLab 机器人 Token，仅 GitLab 配置使用 |
| `bot.gitlab.bot_username` | string | 校验 Token 时获取的用户名 |
| `bot.github.app_id` | int | GitHub App ID，仅 GitHub 配置使用 |
| `bot.github.private_key` | string | GitHub App PEM 私钥完整内容 |
| `bot.github.bot_name` | string | 校验 App 时获取的 `<App slug>[bot]` |
| `bot.updated_by` | string | 最近一次保存配置的管理员账号 |
| `bot.updated_at` | int | 最近一次保存时间，Unix 时间戳，单位为秒 |

- `gitlab` 和 `github` 仅保存与代码源类型对应的一项。`configured`、`token_configured`、`private_key_configured` 由保存的配置生成，不重复存储。
- 存储模型的 `Bot` 字段使用 `bson:"bot,omitempty"`、`json:"-"`、`yaml:"-"`；代码源接口使用独立的请求和响应类型承载 `bot`。对外响应只包含配置状态和身份信息，不能直接序列化存储中的机器人凭证。
- 创建时在同一条代码源记录中写入 `bot`。更新时，在现有代码源字段的同一次 MongoDB 更新中，按请求情况追加 `$set bot`、`$unset bot` 或保持不变，避免分两次写入导致部分保存成功。
- 原授权字段和代码源顶层 `updated_at` 沿用现有代码源更新规则；不因机器人配置变更额外改写 OAuth Token 或强制更新顶层 `updated_at`，后者参与 GitLab OAuth Token 刷新判断。机器人修改时间单独记在 `bot.updated_at`。
- 未提交 `bot` 的代码源更新、OAuth 回调和 Token 刷新保留 `bot` 字段；删除代码源后，随原记录的 `deleted_at` 一起失效。

### 配置接口处理

复用 `pkg/microservice/systemconfig/core/codehost/handler/router.go` 中现有的代码源 GET、POST、PATCH 路由。POST、PATCH 沿用 `NewContextWithAuthorization` 和 `ctx.Resources.IsSystemAdmin` 校验，GET 沿用现有查询权限和 `encryptedKey` 处理。不提供独立的机器人 GET、PATCH、DELETE 接口，也不单独写入机器人配置。

请求绑定时必须区分 `bot` 未传、显式 `null` 和配置对象，不能用一个普通指针将前两种情况都当成 `nil`。原代码源字段继续使用现有绑定、校验和更新规则，不借本次改动重写其他字段的 PATCH 语义。

**保存流程**

1. 创建时完成原代码源创建校验；更新时根据路径 ID 查询未删除的记录，完成原更新校验。更新目标不存在时转换为 `ErrNotFound`，返回 HTTP 404。
2. 根据请求中 `bot` 的情况决定保留、清除或合并配置。提交对象时校验系统级集成、代码源类型和字段；首次配置需要完整凭证，GitHub App ID 变更时必须同时提供新私钥。
3. 需要保存机器人配置时，用合并后的凭证请求 GitLab 当前用户信息或 GitHub App 信息；网络设置取本次保存后生效的代码源值。未传 `bot` 或传 `null` 时不执行机器人凭证校验。
4. 全部校验成功后，记录机器人用户名或 App 名称、修改人和机器人修改时间，将代码源字段与机器人配置一次写入；任一步校验失败均不写入。
5. 将保存结果转换为代码源响应对象，保留原字段，并附加本文定义的脱敏 `bot` 响应。

详情 GET、列表 GET 的每一项，以及 POST、PATCH 的返回值均使用相同的脱敏 `bot` 结构。内部 HTTP 接口继续禁止返回机器人凭证；不向调用方返回 Token 或私钥的加密值。

Token 和私钥不得写入请求日志、操作日志或错误信息。现有 `pkg/middleware/gin/request_log.go` 会记录请求体，需覆盖原代码源 POST、PATCH 请求的机器人凭证屏蔽；只屏蔽旧 `/bot` 路径或仅在 Handler 中脱敏均不能阻止原始请求体被记录。

### 内部凭证读取

现有 `pkg/shared/client/systemconfig/codehost.go` 中的 `GetCodeHost()` 直接调用代码源 Service 读取数据库，不发起 HTTP 请求。本次复用该调用方式，将机器人配置传递给评论发布逻辑，无需新增返回凭证的 HTTP 接口。

在内部 `systemconfig.CodeHost` 中增加 `Bot` 字段，并在 `GetCodeHost()` 的结果映射中赋值。该内部字段同样禁止 JSON、YAML 序列化；代码源接口返回的 `bot` 字段仅包含脱敏后的配置状态和身份信息。

机器人凭证只在后端创建评论客户端时使用，不写入工作流任务参数、AI 提示词或 JobExecutor 执行环境。

### AI 审查评论发布

现有发布入口为 `aiReviewReportCtl.AfterRun()`，使用任务中的 `CodehostID`、仓库信息和 PR/MR 编号调用 `scmnotify.PublishAIReviewReport()`。本次在该发布流程中读取机器人配置，前端无需逐次传入评论身份。

**发布流程**

1. 在一次 `PublishAIReviewReport()` 中读取一次代码源及机器人配置，按本文的身份规则创建客户端。
2. 将该客户端用于行内评论和总结评论，复用现有的评论内容生成和行号定位逻辑。避免两个入口分别读取配置，导致发布期间配置变化时出现两种作者身份。
3. 行内评论仍先发布，总结评论仍后发布；未能发布为行内评论的 Findings 按现有逻辑汇总到总结评论，两者使用同一身份。
4. 凭证失效、权限不足或 App 未安装时记录错误，沿用现有发布失败处理，不切换其他身份。保持现有 `AfterRun()` 记录警告的行为，不额外将审查任务判为失败。

**GitLab 客户端**

- 已配置机器人时，使用 `github.com/xanzy/go-gitlab` 的 `NewClient()` 创建 Access Token 客户端，并设置代码源地址及带代理、TLS 配置的 HTTP Client。
- 不调用现有 `pkg/tool/git/gitlab/client.go` 中的 `NewClient()`；该函数会执行 `UpdateGitlabToken()`，重新读取并刷新代码源 OAuth Token，可能将机器人 Token 替换为原身份的 Token。
- 总结评论复用 `Notes.CreateMergeRequestNote()`，行内评论复用 `Discussions.CreateMergeRequestDiscussion()`。评论作者由 Token 所属用户决定，不由 `bot_username` 指定。
- 未配置机器人时保留现有客户端创建方式及 OAuth 刷新行为。

**GitHub 客户端**

1. 已配置机器人时，使用该代码源的 App ID 和私钥创建 App 认证客户端，复用现有 `ghinstallation` 认证和安装查询逻辑。
2. 根据 `repoOwner` 查找该 App 的 Installation，使用 Installation 认证客户端自动获取、刷新 Installation Token。App 必须安装到目标账号，并包含目标仓库的访问权限。
3. 总结评论复用 `Issues.CreateComment()`，行内评论复用 `PullRequests.CreateReview()`；二者均通过 Installation Token 认证，以该 App 的机器人身份发布。
4. 未配置机器人时，保留现有的全局 GitHub App 优先、无全局 App 时使用代码源 Token 的行为。

复用 `pkg/tool/git/github/client.go` 的 App 能力时，需要补齐以下适配：

| 适配项 | 实现说明 |
| ------ | -------- |
| 私钥格式 | HTTP 请求接收 PEM 原文；现有 `NewAppClient()` 的 `AppKey` 参数使用 Base64，在调用边界转换，Base64 仅用于格式适配，不作为加密 |
| 网络设置 | 校验 App、获取 Installation Token 和发布评论使用代码源的代理及 TLS 设置；查询 Installation 沿用现有 `getInstallationID()` 的逻辑，不使用代理 |
| App 来源 | 显式传入代码源机器人配置，不通过 `GetGithubAppClientByOwner()` 读取全局 App；该函数继续用于未配置机器人的原有分支 |

### 涉及模块

以下路径均相对于 Zadig 仓库根目录。

| 模块 | 路径 | 改动说明 |
| ---- | ---- | -------- |
| 配置接口 | `pkg/microservice/systemconfig/core/codehost/handler/` | 扩展原查询、创建和更新接口，区分 `bot` 未传、`null` 和对象，输出脱敏响应 |
| 配置逻辑 | `pkg/microservice/systemconfig/core/codehost/service/` | 复用原代码源校验，合并机器人配置、校验凭证后统一保存 |
| 配置存储 | `pkg/microservice/systemconfig/core/codehost/repository/` | 增加 `bot` 模型，并入原代码源的单次写入 |
| 内部读取 | `pkg/shared/client/systemconfig/codehost.go` | 将机器人配置传递给 AI 评论发布逻辑 |
| 评论发布 | `pkg/microservice/aslan/core/common/service/scmnotify/` | 一次发布共用配置和客户端，接入两种评论 |
| GitHub App | `pkg/tool/git/github/client.go` | 复用安装认证，适配私钥格式及网络设置 |
| 请求日志 | `pkg/middleware/gin/request_log.go` | 屏蔽原代码源 POST、PATCH 请求中的机器人凭证 |

### 验收要点

| 场景 | 预期结果 |
| ---- | -------- |
| 打开系统级 GitLab、GitHub 代码源高级配置 | 从原详情或列表响应的 `bot` 回显配置状态和身份信息，凭证输入框为空 |
| 新建代码源时一并配置机器人 | 一次 POST 保存代码源和机器人配置，任一校验失败均不创建 |
| 编辑代码源并更新机器人配置 | 一次 PATCH 保存两者；机器人校验失败时，代码源其他字段也保持原值 |
| 只修改代码源其他字段，未传 `bot` | 保留原机器人配置，不额外发起机器人凭证校验 |
| 更新已配置机器人，未填写 Token 或私钥 | 使用原凭证；App ID 变更时缺少新私钥则拒绝保存 |
| 传入 `bot: null` 后保存 | 清除机器人配置并正常保存代码源其他字段；未配置时重复清除也成功 |
| 传入 `bot: {}` | 返回参数错误，原代码源和机器人配置均不变 |
| GitLab 配置机器人 Token 后执行 AI 审查 | 总结评论和行内评论的作者均为 Token 所属用户；原 OAuth Token 刷新不覆盖机器人配置 |
| GitHub 配置并安装 App 后执行 AI 审查 | 总结评论和行内评论的作者均为该 App 的机器人，即使存在全局 App 也使用本代码源配置 |
| 机器人凭证无效或无目标仓库权限 | 输出明确错误，不以代码源账号或全局 App 的身份重新发布 |
| 更新配置未通过凭证校验 | 代码源和机器人旧配置均保留，后续评论继续使用旧配置 |
| 删除机器人配置 | 后续 AI 审查恢复原有身份选择规则 |
| 未配置机器人或执行其他代码源操作 | 保持原有行为 |
| 检查 HTTP 响应、请求日志和工作流数据 | 均不包含机器人 Token 或私钥 |

## 附录：凭证准备

### GitLab Token

GitLab 评论作者由 Token 所属账号决定。支持群组 Token、Service Account 的 Personal Access Token（PAT）以及专用账号 PAT，均填入同一个 `bot.gitlab.token` 字段，前端无需增加 Token 类型选择。

Service Account 是服务账号，使用该账号的 PAT 认证；它与 Group Access Token 自动生成的群组机器人不是同一种账号。几种方式的区别如下：

| 对比项 | Group Access Token | Service Account 的 PAT | 专用账号 PAT | Project Access Token |
| ------ | ------------------ | --------------- | ------------ | -------------------- |
| 可用条件 | 自建免费版可用；GitLab.com 需 Premium / Ultimate | GitLab.com 支持 Free / Premium / Ultimate；自建 Free 从 18.11 起正式支持，旧版本需按版本和套餐确认 | 按实例账号策略创建普通账号 | 自建免费版可用；GitLab.com 需 Premium / Ultimate |
| 评论身份 | 群组机器人用户 | 服务账号 | 普通用户 | 项目机器人用户 |
| 覆盖范围 | 所属群组及其子群组、项目；账号不能直接加入其他群组或项目 | 实例级或顶级群组创建的服务账号可被邀请到其他群组、项目；子群组或项目创建的账号范围受限 | 被加入的所有群组和项目 | 仅所属项目 |
| 占用许可席位 | 否 | 否 | 按普通用户规则计费 | 否 |
| 创建方式 | 群组 Owner 在群组设置中创建 | 支持 UI 或 API；GitLab.com 由顶级群组 Owner 创建，自建默认由管理员创建，也可授权群组 Owner | 创建普通账号并加入群组 | 项目 Maintainer / Owner 在项目设置中创建 |

选择建议：

- 代码源下的仓库集中在同一个顶级群组：使用 Group Access Token
- 代码源下的仓库分布在多个顶级群组：优先使用实例级或顶级群组创建的 Service Account，将同一账号加入各目标群组或项目，使用同一份 PAT
- 当前 GitLab 版本或实例策略不支持 Service Account：使用专用账号 PAT
- 不建议使用 Project Access Token，一个代码源只能配置一个 Token，无法覆盖多个项目

**Service Account 的创建与授权**

以下以 GitLab.com 顶级群组为例；自建实例由管理员在 Admin → Settings → Service accounts 管理，允许群组 Owner 创建时也可使用群组入口。旧版本可按官方 API 文档操作。

1. 使用顶级群组 Owner 账号，进入 Settings → Service accounts → Add service account，设置名称（如 `Zadig AI Review`）和用户名。
2. 在服务账号菜单中选择 Manage access tokens → Add new token，选择 `api` scope，按实例策略设置到期时间，创建并保存 PAT。
3. 在每个目标群组或项目的 Manage → Members → Invite members 中添加该服务账号，授予至少 Reporter 角色。账号不会自动拥有其他群组或项目的访问权限。
4. 将这份 PAT 填入 Zadig 代码源高级配置中的机器人 Token 字段，随代码源一起保存。评论作者为该服务账号，PAT 的名称不决定评论显示名。

**Service Account 的范围与套餐限制**

- 顶级群组创建的服务账号可以被邀请到其他群组或项目；子群组创建的服务账号仅限该子群组及其下级范围，项目创建的服务账号仅限所属项目。
- 目标顶级群组强制 SSO 时，只能邀请该顶级群组或其子群组、项目创建的服务账号，不能据此承诺同一账号跨所有顶级群组通用。
- GitLab.com Free 每个顶级群组最多创建 100 个服务账号；自建 Free 整个实例最多 100 个。Premium / Ultimate 不限制数量，服务账号不占用户席位。
- 自建 Free 从 GitLab 18.11 起正式支持服务账号；18.10 需启用 `service_accounts_available_on_free_or_unlicensed` 功能开关。使用旧版本时应先确认是否支持及管理员是否开放创建权限。

**Group Access Token 与凭证轮换**

- Group Access Token 适合单个群组及其下级仓库，创建时选择 `api` scope 和至少 Reporter 角色；Token 名称会作为其自动生成的群组机器人显示名。
- 在 Token 到期前使用 GitLab 的轮换（rotate）功能并更新 Zadig 配置。Group Access Token 轮换保留原机器人用户，重新创建会生成新的机器人用户。
- Service Account 的 PAT 轮换或重新创建后，只要仍属于同一服务账号，评论作者身份不变；不要把 PAT 名称当成服务账号名称。

以上套餐与范围说明核对于 2026-10-20。参考：[Service accounts](https://docs.gitlab.com/user/profile/service_accounts/)、[Service accounts API](https://docs.gitlab.com/api/service_accounts/)、[Group access tokens](https://docs.gitlab.com/user/group/settings/group_access_tokens/)。

### GitHub App

在 GitHub 中依次进入 Settings → Developer settings → GitHub Apps → New GitHub App 创建应用，关键配置如下：

| 配置项 | 说明 |
| ------ | ---- |
| GitHub App name | 即评论作者显示名，如 `zadig-ai-review` |
| Webhook | 取消勾选 Active，机器人身份不需要接收事件 |
| Repository permissions | Pull requests：Read and write；Issues：Read and write；Contents：Read-only；Metadata：Read-only |
| Where can this GitHub App be installed | 仅本组织使用选择 Only on this account；需要安装到多个组织选择 Any account |

创建后：

1. 在 App 设置页记录 App ID
2. 在 Private keys 区域生成并下载私钥文件
3. 进入 Install App，将 App 安装到需要 AI 代码审查的组织或仓库

注：同一个 GitHub App 可以安装到多个组织或个人账号下，Zadig 会按仓库自动查找对应的安装。
