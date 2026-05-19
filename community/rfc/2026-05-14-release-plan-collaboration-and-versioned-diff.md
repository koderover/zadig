# 发布计划多人协作与版本化变更展示方案

- 作者：KodeRover
- 关联 Issue：TBD
- 日期：2026-05-14
- 评审人：TBD
- 评审状态：pending

## 目标

这次方案同时解决发布计划的两个需求：

- 多人协作编辑：用户编辑发布计划时，可以看到有哪些人正在编辑哪些内容。
- 操作记录细化：用户查看发布计划操作记录时，可以看到这次保存具体改了哪些工作流任务参数。
- 版本记录：每次配置保存后记录一个版本，版本里只保存这次编辑区块的输入参数快照，后续查看“这次改了什么”时，用这次编辑的前后快照做对比。

## 一句话方案

不做强锁，也不阻止多人同时编辑。用户编辑时，前端实时展示“谁正在编辑什么”；用户保存后，后端记录一个“编辑区块输入参数版本”；用户点开操作记录详情时，后端比较这次编辑区块的前后快照，把变化整理成按发布内容和工作流任务分组的可读详情。

## 用户能看到什么

### 正在编辑提示

用户进入发布计划详情页后，不会默认显示“正在编辑”。只有当某个用户真正点击某块内容的编辑入口后，其他用户才会看到提示。

第一版建议展示这些编辑区块：

- 基础信息：名称、负责人、发布窗口、定时执行、需求关联。
- 审批配置。
- 某一个发布内容。

示例：

```text
huanghongbo 正在编辑基础信息
patrick 正在编辑发布内容 log-test
2 人正在编辑审批配置
```

### 操作记录详情

操作记录列表仍然先展示一句摘要：

```text
huanghongbo 更新发布内容 log-test
```

用户点开详情后，再展示这次保存具体改了什么：

```text
发布内容：log-test

构建任务：build
- 代码分支：main -> release/202605
- 镜像标签：v1.2.3 -> v1.2.4

Apollo 任务：update-config
- 命名空间：application -> application-prod
- DB_HOST：10.0.0.1 -> 10.0.0.2

DMS 任务：data-change
- SQL 内容：已变更
```

大文本内容，比如脚本、SQL、大段 YAML 或 JSON，第一版默认只展示“已变更”，不在普通操作记录里展开全文。

敏感字段沿用工作流本身已有的敏感变量配置，例如 keyvault 的 `is_sensitive` 和工作流变量里的 `is_credential`，只展示“已变更”，不返回原始值。

## 前端需要支持什么

### 进入编辑态

- 用户打开发布计划详情页时，只建立 WebSocket 连接，不立即进入编辑态。
- 用户点击某个编辑入口后，前端再告诉后端“我正在编辑这一块”。
- 编辑区块建议和后端保持一致：`metadata`、`approval`、`job:<jobID>`。
- 页面上哪些状态可以编辑、哪些入口显示，由前端根据发布计划状态和权限判断；后端收到请求时会再做一次校验。

### 维护编辑会话

前端需要在一次区块编辑期间维护同一个 `session_id`：

- 用户进入某个编辑区块时生成或获取一个 `session_id`。
- 该编辑区块里的所有保存请求都带同一个 `session_id`。
- 每 10 到 15 秒发送一次心跳，告诉后端“我还在编辑”。
- 用户取消编辑、关闭弹窗、保存完成或切换编辑区块时，通知后端离开当前编辑态。

### 保存配置

现有 `verb + spec` 保存方式继续保留。前端在调用保存接口时，需要额外带上 `session_id`：

```json
{
  "verb": "update_release_job",
  "spec": {},
  "session_id": "uuid"
}
```

这样后端可以知道这几次 `verb` 保存属于同一次区块编辑。

### 提交版本

如果采用“按编辑区块合并版本”的方式，前端需要在一次区块编辑完成时调用版本提交接口：

```json
{
  "session_id": "uuid",
  "section_key": "job:job-id"
}
```

建议触发时机：

- 用户点击编辑弹窗里的“保存”或“确定”。
- 用户关闭编辑弹窗时，如果已经有变更，也需要触发提交。
- 用户切换到另一个编辑区块前，如果当前区块已有变更，也需要先提交当前区块。

这样可以避免一个区块里多次 `verb` 保存生成多个版本。

### 展示版本差异

操作记录列表仍然先展示摘要。用户点开某条操作记录详情时：

- 如果该记录包含 `from_version` 和 `to_version`，前端调用版本差异接口。
- 前端按返回的 `groups` 渲染变更详情。
- 大文本字段如果标记为 `large_text`，默认展示“已变更”。
- 敏感字段如果在原始配置里标记为敏感变量，只展示“已变更”，不展示原始值。
- 如果历史记录没有版本信息，前端继续按旧展示方式处理。

## 后端需要支持什么

### 实时协作编辑态

后端为每个浏览器编辑会话维护一条编辑记录。建议第一版的编辑粒度按内容块划分：

- `metadata`：基础信息，比如名称、负责人、发布窗口、定时执行、需求关联。
- `approval`：审批配置。
- `job:<jobID>`：某一个发布内容。

编辑记录建议包含：

```go
type ReleasePlanEditingSession struct {
    PlanID           string `json:"plan_id"`
    SessionID        string `json:"session_id"`
    UserID           string `json:"user_id"`
    UserName         string `json:"user_name"`
    Account          string `json:"account"`
    IdentityType     string `json:"identity_type,omitempty"`
    Avatar           string `json:"avatar,omitempty"`
    SectionKey       string `json:"section_key"`
    SectionType      string `json:"section_type"`
    SectionName      string `json:"section_name"`
    BaseVersion      int64  `json:"base_version"`
    EditingStartedAt int64  `json:"editing_started_at"`
    LastHeartbeatAt  int64  `json:"last_heartbeat_at"`
}
```

说明：

- `BaseVersion` 表示用户开始编辑时看到的发布计划版本。第一版只用于前端提示，不用于阻断保存。
- `SectionType` 表示正在编辑哪类内容，前端可以直接判断这是基础信息、审批还是发布内容。
- `IdentityType` 和 `Avatar` 是可选增强字段，便于前端直接展示“谁正在编辑”。
- `EditingStartedAt` 表示真正进入编辑态的时间，不等同于页面建立连接的时间。

编辑态使用 Redis 保存，并设置自动过期时间。这样即使浏览器异常关闭、断网、服务端连接断开，编辑态也会自动消失，不会一直显示“某人在编辑”。

如果 Aslan 是多副本部署，需要用 Redis 做一次“跨 Pod 通知”。某个 Pod 收到用户编辑态变化后，先写 Redis，再发一条 Redis 消息；其他 Pod 收到这条消息后，再推送给自己本地持有的 WebSocket 连接。这样连接在不同 Pod 上的用户也能互相看到编辑状态。

`hook` 外部系统配置本身不纳入第一版多人协作提示范围。外部 `hook` 触发后，发布计划可能进入“外部检测”阶段，这个阶段是否还能编辑，由前端决定是否展示编辑入口；后端只负责兜底校验。

### 保存与版本化

当前后端已经有统一的配置更新接口：

- `PUT /api/release_plan/v1/:id`
- 请求体使用 `verb + spec` 表示“这次改的是哪一块内容”

这一套机制建议继续保留，不需要为了版本化把它推翻重做。原因有三个：

- 现有权限判断就是按 `verb` 分开的。
- 现有操作记录也是按 `verb` 生成摘要。
- 第一版版本化只需要挂在“配置保存成功”这个时机上，不要求接口形态改变。

保存规则保持“最后保存生效”：

- 不加硬锁。
- 不因为其他人正在编辑就拒绝保存。
- 多人保存同一块内容时，以最后一次成功保存的结果为准。

### 版本和保存次数

这里需要和前端约定清楚：

- 版本是按“成功保存一次配置”生成的，不是按“产生一条操作记录”生成的。
- 配置类保存包括：名称、负责人、时间窗口、审批、发布内容增删改排等。
- 流程类动作不生成配置版本，比如：状态流转、审批通过/拒绝、执行、重试、跳过、外部检测回调。

如果当前前端交互是“用户改一块、点一次保存、发一个 `verb` 请求”，那就自然是一条配置保存对应一个版本。

如果采用“按编辑区块合并版本”，则同一个 `session_id` 下的多次 `verb` 保存，最终合并成一个版本。

如果后面前端希望改成“整页统一保存”：

- 可以把多个改动合并后一次提交。
- 后端一次性应用这些改动。
- 最终只生成一个版本。

这属于前端交互方式变化，不影响版本模型本身。第一版可以先兼容现有单 `verb` 保存模式，后续如果真的需要整页保存，再单独补一个批量保存接口。

### 版本生成时机

- 用户点击保存配置时，生成新的配置版本。
- 审批通过、审批拒绝、进入执行、执行完成、外部检测等自动流转，默认只记录状态事件，不生成新的配置版本。
- 如果未来某个系统流程会直接改动发布计划配置本身，再单独评估是否补充“系统生成版本”。

### 发布计划版本模型

新增发布计划版本集合。这里不保存完整发布计划，而是只保存“这次编辑区块的输入参数快照”。建议模型：

```go
type ReleasePlanVersion struct {
    ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    PlanID       string             `bson:"plan_id" json:"plan_id"`
    BaseVersion  int64              `bson:"base_version,omitempty" json:"base_version,omitempty"`
    Version      int64              `bson:"version" json:"version"`
    Operator     string             `bson:"operator" json:"operator"`
    Account      string             `bson:"account" json:"account"`
    SectionKey   string             `bson:"section_key" json:"section_key"`
    SectionName  string             `bson:"section_name" json:"section_name"`
    Verb         string             `bson:"verb" json:"verb"`
    BaseSnapshot interface{}        `bson:"base_snapshot,omitempty" json:"base_snapshot,omitempty"`
    Snapshot     interface{}        `bson:"snapshot" json:"snapshot"`
    CreatedAt    int64              `bson:"created_at" json:"created_at"`
}
```

说明：

- `SectionKey` 表示这个版本对应哪个编辑区块，例如 `metadata`、`approval`、`job:<jobID>`。
- `BaseVersion` 表示这次编辑会话开始时看到的版本号。
- `BaseSnapshot` 表示该区块开始编辑时的输入参数快照。
- `Snapshot` 表示该区块保存完成后的输入参数快照。

快照里只保留输入参数，不保留运行态和执行态字段。例如：

- 基础信息版本只保留名称、负责人、时间窗口、定时执行、需求关联、Jira 关联等输入项。
- 审批版本只保留审批配置输入，不保留审批实例运行状态。
- 发布内容版本只保留该发布内容的输入参数，不保留 `status`、`task_id`、`executed_by`、`executed_time` 这类运行字段。

这样可以避免因为单条发布计划过大导致版本体积和 diff 计算成本失控。

发布计划主表中也建议增加当前版本号：

```go
Version int64 `bson:"version" json:"version"`
```

历史发布计划默认版本可以是 `0`。升级后第一次保存生成 `version = 1`。

### 操作日志关联版本

现有发布计划操作日志需要关联版本。建议给 `ReleasePlanLog` 增加：

```go
FromVersion int64 `bson:"from_version,omitempty" json:"from_version,omitempty"`
ToVersion   int64 `bson:"to_version,omitempty" json:"to_version,omitempty"`
```

操作记录列表仍然保持简洁，例如：

```text
2026-05-14 10:00 huanghongbo 更新发布内容 log-test v12 -> v13
```

用户点击详情时，前端使用 `plan_id`、`from_version`、`to_version` 请求版本差异。

这里的 `from_version` 不一定总是“上一个版本”。它表示这次编辑会话开始时看到的版本。

例如：

- A 基于 `v1` 开始编辑某个发布内容。
- B 先保存，生成 `v2`。
- A 继续编辑后再保存，生成 `v3`。

那 A 这条操作记录应该关联：

- `from_version = 1`
- `to_version = 3`

这样点开详情时，看到的是这次编辑会话对应的区块变更区间，而不是简单的 `v2 -> v3`。

这里的版本和状态事件职责分开：

- 版本主要回答“这次保存后的配置是什么”。
- 操作日志和状态日志继续回答“发布计划后来经历了什么流转”。
- 即使自动流转不生成版本，用户仍然可以从最近一次配置版本里查看当时保存下来的区块输入参数。

## 变更计算

变更计算负责比较一次编辑区块版本的前后快照。设计上先保证所有输入参数变化都能被找出来，再把结果整理成用户能看懂的字段名和分组。

第一版建议在用户查看详情时再计算变更内容，而不是在保存时就提前算好：

- 保存成功时，只负责保存新版本、写操作日志、广播版本变更。
- 用户点击某条操作记录详情时，后端再根据 `from_version` 和 `to_version` 读取该次编辑区块的前后快照并计算变更内容。
- 第一版先不做变更结果缓存；如果后续确认详情打开比较慢，再单独评估缓存或后台提前计算。

处理流程：

1. 读取 `to_version` 对应版本里的 `BaseSnapshot` 和 `Snapshot`。
2. 从外到内逐层比较字段。
3. 数组优先按“能代表这个元素身份的字段”匹配。
4. 生成基础差异项。
5. 对差异项做脱敏、分组和字段翻译。

### 对比前的数据整理

由于版本里保存的是区块输入参数快照，真正对比之前只需要再过滤少量不适合展示的字段，例如敏感信息和大文本字段，不需要再从整份运行对象里剔除执行状态。

### 数组匹配

数组不能总是按下标比较。能识别元素身份的数组，应该按身份字段对齐：

```text
发布内容：id
工作流阶段：name
工作流任务：name + type
工作流参数：name
代码仓库：source + repo_namespace + repo_name + remote_name
构建服务：service_name + service_module
部署服务：service_name + service_module
key/value 数组：key
```

如果某类数组没有明确的身份字段，再退回按下标比较。

### 字段规则管理

字段中文名、忽略哪些字段、哪些字段需要脱敏，这些规则可以放在一张统一的规则表里管理。实现上可以用普通的路径映射表；如果规则特别多，再考虑用 `trie` 这种“按字段路径快速查规则”的结构。

这里的 `trie` 只适合做“某个字段路径该怎么处理”的查找，不适合拿来做两个版本内容是否有差异的核心计算。真正找出变化的步骤，还是靠前面的逐层比较。

## API 变更

### 协作态

```http
GET /api/aslan/release_plan/v1/:id/collaboration/ws
GET /api/aslan/release_plan/v1/:id/collaboration/editors
```

`editors` 用于页面初始化和 WebSocket 断线重连后的状态恢复。

### 版本

```http
GET /api/aslan/release_plan/v1/:id/versions
GET /api/aslan/release_plan/v1/:id/versions/:version
GET /api/aslan/release_plan/v1/:id/versions/:from/diff?to=:to
POST /api/aslan/release_plan/v1/:id/versions/commit
```

`commit` 接口用于告诉后端“这次区块编辑结束了，可以把这组变更合成一个版本”。

### 操作日志

现有日志接口保持不变，但返回值增加版本信息：

```http
GET /api/aslan/release_plan/v1/:id/logs
```

每条日志可以包含 `from_version`、`to_version`。

## 向后兼容

- 现有发布计划 API 保存语义不变。
- 历史发布计划默认版本号为 `0`。
- 没有版本信息的历史操作日志仍按旧格式展示。
- 升级后第一次保存生成第一个版本。
- 版本差异展示是新增能力，不影响现有保存流程。

## 性能考虑

- 版本只保存编辑区块的输入参数快照，不保存整份发布计划运行对象。
- 数组先按身份字段对齐后再比较，避免两两查找导致耗时变长。
- 大文本字段在普通响应里只标记“已变更”，不做全文对比。
- 如果一个内容块本身完全没变，就直接跳过，不继续往下比。
- 用户点开操作详情时再计算变更内容，避免把比较耗时放到保存流程里。
- 后续如果遇到大对象性能问题，可以先比较每个工作流任务输入参数是否变化；没变化的任务就不继续往下比。
- 第一版先不做结果缓存，等确认真的有明显性能压力，再考虑补。

## 安全与隐私

- 敏感字段在返回前必须脱敏，脱敏依据沿用工作流自身已经配置好的敏感变量标记，不额外定义审批人、手机号之类的特殊字段规则。
- 脱敏规则在生成基础差异项后、返回给前端前执行。
- 不通过版本差异接口暴露敏感变量原始值。
- WebSocket 接口需要复用发布计划查看/编辑权限校验。

## 实施拆分

虽然产品目标是一口气交付完整体验，工程实现仍建议按模块拆：

1. 增加版本模型，并在保存成功后记录编辑区块的输入参数快照。
2. 增加 WebSocket 协作态，使用 Redis 自动过期和跨 Pod 通知。
3. 增加变更计算能力，支持数组按身份字段匹配和敏感字段脱敏。
4. 增加翻译后的变更详情返回结构，并和操作日志关联。
5. 首版尽量一次性覆盖已知字段中文标签，未知字段用处理后的字段路径兜底展示，但不改变核心返回格式。
