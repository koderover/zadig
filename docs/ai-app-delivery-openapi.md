# AI 应用交付 OpenAPI 设计

## 文档说明与范围

本文统一维护本需求涉及的 OpenAPI 设计。**状态：接口已实现，待平台联调验收。** 当前范围为工作流模板列表、按模板创建工作流、工作流详情的必要字段补齐，以及绑定增量更新。

Query 参数使用 `camelCase`，JSON 字段使用 `snake_case`，成功响应直接返回业务结构，不增加 `data` 包装。

参数的必填、默认值和空值处理沿用后端现有行为，条件必填在表中明确标注。新增接口复用对应业务逻辑；此前已明确的新绑定更新接口的资源定位参数单独说明。

项目、环境及授权由用户侧负责准备。服务、构建导入复用已有模板创建接口，之后创建或更新绑定工作流，再通过已有执行接口发布。管理员负责保证发布模板包含合适的构建、非生产部署任务，首次部署应包含服务配置，不能只更新镜像。

本期不新增模板详情查询，也不新增模板管理、项目创建、配置漂移检测或 `templates.contractVersion`。列表提供创建需要的模板 ID 与版本，具体任务从创建后的工作流详情读取。

## 接口概览

| 能力 | 请求 | 处理方式 |
| --- | --- | --- |
| 查询工作流模板列表 | `GET /openapi/templates/workflows` | 新增，返回模板预览及可绑定版本 |
| 按模板创建工作流 | `POST /openapi/workflows/custom` | 复用，补齐模板展开、校验和保存链路 |
| 查询工作流详情 | `GET /openapi/workflows/custom/<name>/detail?projectKey=<项目标识>` | 复用，仅增加绑定和稳定阶段 ID 等必要字段 |
| 替换工作流绑定增量 | `PUT /openapi/workflows/custom/<name>/template-binding?projectKey=<项目标识>` | 新增，只接收完整增量集合，实例配置由服务端保留 |

本次需补齐的行为包括：绑定创建时统一展开、校验、保存并锁定选定版本；详情补齐绑定字段并按基准渲染；新绑定更新接口按项目和名称定位、保留实例配置，只在当前基准版本上更新增量，不自动升级模板。

新 PUT 替换绑定子资源中可编辑的完整增量集合，绑定身份和版本由服务端维护；它不是向旧增量追加操作，也不是专用的“加入服务”接口。原 `PUT /openapi/workflows/custom/<name>` 继续保留完整工作流更新契约，本流程不再要求 CLI 使用它回写整份定义。

上述定义操作均不触发工作流任务。成功沿用 HTTP `200`，不另建 `201/409` 约定。现有详情路径的 `/detail` 保持不变。

## 认证与权限

支持 OAuth 登录和 API Token 两种认证方式，均以当前用户身份请求 Zadig，权限由后端现有规则判断，不分发管理员共享凭据。已有接口沿用后端当前鉴权行为；新增接口复用对应内部接口的权限规则，不新增权限类型或扩大权限范围。

| 操作 | 沿用规则 | 状态 |
| --- | --- | --- |
| 查询模板列表 | 系统管理员、系统模板查看权限；或指定项目的项目管理员、工作流创建权限 | 新 OpenAPI 复用内部工作流模板列表的权限规则 |
| 创建工作流 | 系统管理员、目标项目的项目管理员或工作流创建权限 | 与现有创建 OpenAPI 一致 |
| 查询工作流详情 | 沿用现有详情 OpenAPI 的鉴权行为 | 当前未做工作流对象权限检查；本次不增加新的查看或协作权限规则 |
| 替换绑定增量 | 系统管理员；或所属项目的项目管理员、工作流编辑权限、该工作流协作编辑权限 | 新接口复用内部绑定编辑的权限规则；非系统管理员须有该项目的授权信息 |

模板是系统级资源，列表的 `projectKey` 只用于在指定项目中创建工作流时的授权，不代表模板归属，也不赋予模板编辑权限。新更新接口按 Query 的 `projectKey` 和路径的 `name` 联合定位工作流，核对实际归属后按真实资源授权，不从 Body 接收资源身份。操作日志记录已授权请求，敏感增量值不得明文写入日志。

## 查询工作流模板列表

**请求**

```text
GET /openapi/templates/workflows?projectKey=<项目标识>&excludeBuildIn=true
```

**Query 参数**

| 参数名 | 类型 | 描述 | 必填 | 默认值 |
| --- | --- | --- | --- | --- |
| `projectKey` | string | 用于权限判断的目标项目；系统管理员或具有系统模板查看权限时可省略，其他用户必须提供 | 条件必填 | 无 |
| `category` | string | 按传入字符串过滤；常用值为 `release`；不传或空字符串不按分类过滤，其他非空值同样按原值查询 | 否 | 无 |
| `excludeBuildIn` | string | 沿用现有 Query 解析：仅字符串 `true` 表示排除内置模板，其他值（包括未传、空字符串及 `false`）均表示不排除 | 否 | `false` |

复用已有小规模配置列表及按模板名称倒序的排序，不增加分页。返回 `workflow_templates` 数组，无结果时为 `[]`。

内部模板列表使用 `projectName` 传递权限判断所需项目；新增 OpenAPI 按当前领域约定使用 `projectKey`，只做参数映射，不修改内部接口或权限规则。筛选参数不增加枚举或布尔格式校验。

**WorkflowTemplateBrief 字段**

| 参数名 | 类型 | 描述 |
| --- | --- | --- |
| `id` | string | 模板 ID，24 位十六进制字符串 |
| `template_name` | string | 系统内唯一的模板名称 |
| `description` | string | 模板描述 |
| `category` | string | 普通工作流为空字符串，发布工作流为 `release` |
| `build_in` | bool | 是否系统内置模板 |
| `latest_version` | int | 从版本快照集合查询的最新版本；无快照为 `0`，不可用于绑定创建 |
| `update_by` | string | 更新人，沿用现有预览字段名 |
| `update_time` | int | 更新时间，Unix 秒 |

仅返回以上预览字段，不返回完整任务配置、凭据或页面多语言映射。GET 不生成快照，也不以当前模板内容冒充版本快照。

**返回示例**

```json
{
  "workflow_templates": [
    {
      "id": "690000000000000000000001",
      "template_name": "ai-web-publish",
      "description": "Web 应用构建与非生产环境部署",
      "category": "",
      "build_in": false,
      "latest_version": 3,
      "update_by": "admin",
      "update_time": 1793577600
    }
  ]
}
```

## 按模板创建工作流

**请求**

```text
POST /openapi/workflows/custom
Content-Type: application/json
```

沿用现有 Handler 的 YAML 解析方式，兼容 JSON；现有操作日志的请求体类型为 YAML。新增绑定更新接口则使用 JSON 解析，不改变旧接口的解析方式。

下表仅描述按模板绑定创建的请求。普通工作流创建不要求 `template_binding`；缺失该字段或 `enabled` 未设为 `true` 时，现有后端按普通创建处理，不会自动启用绑定。

**Body 参数**

| 参数名 | 类型 | 描述 | 必填 |
| --- | --- | --- | --- |
| `name` | string | 工作流标识；大小写字母、数字和中划线，按现有业务链要求全局不重名 | 是 |
| `display_name` | string | 展示名称；当前后端允许省略或为空，不自动取工作流标识作为存储值 | 否 |
| `project` | string | 目标项目标识 | 是 |
| `description` | string | 工作流描述 | 否 |
| `template_binding` | TemplateBindingInput | 按模板绑定创建时提供，字段见下表 | 条件必填（绑定创建） |

**TemplateBindingInput 字段**

| 参数名 | 类型 | 描述 | 必填 | 默认值 |
| --- | --- | --- | --- | --- |
| `enabled` | bool | 绑定创建必须显式设为 `true`；未提供时为 `false`，不会启用绑定 | 是（绑定创建） | `false` |
| `template_id` | string | 模板 ID，传入时须为 24 位十六进制字符串；当前后端未显式校验非空 | 否（当前未校验非空） | 空字符串 |
| `base_version` | int | 正数指定快照；未提供或为 `0` 时选最新版本；当前后端对负数也按最新版本处理 | 否 | `0` |
| `delta_patches` | []JSONPatchOperation | 相对模板的完整增量；创建时缺失或 `null` 沿用现有行为归一为 `[]` | 否 | `[]` |

服务端从所选快照取得 `category`、`params`、`stages`、`share_storages`、`concurrency_limit`，应用增量后校验并保存；调用方无需复制模板定义。绑定继承字段以模板加增量为唯一来源，请求中的同名字段不能覆盖渲染结果。

传正数版本时，快照选择、校验、保存必须使用该版本；传 `0` 时在本次创建开始选定最新快照，此后不再次选版。不存在的模板或快照返回错误，不换用其他版本。模板名称、版本快照 ID、创建信息由服务端维护。

调用方应提供明确的 `template_id`，并使用正数版本或 `0`。当前模板 ID 缺失或为空时，模板查询不带 ID 条件，可能选到非预期模板；本文如实保留这一现状，不新增非空校验，也不将其作为推荐用法。负数版本同样保留既有选择最新版本的行为。

**Body 示例**

```json
{
  "name": "pelican-web-publish",
  "display_name": "鹈鹕应用发布",
  "project": "pelican",
  "description": "按管理员模板发布 Web 应用",
  "template_binding": {
    "enabled": true,
    "template_id": "690000000000000000000001",
    "base_version": 3,
    "delta_patches": []
  }
}
```

**返回：HTTP `200`，`{"message":"success"}`。** 重名返回业务错误，不覆盖已有工作流。现有创建包含落库后的回读和更新，报错不保证尚未创建；本接口不承诺事务回滚或幂等键。

## 查询工作流详情

**请求**

```text
GET /openapi/workflows/custom/<name>/detail?projectKey=<项目标识>
```

路径 `name` 与 Query `projectKey` 均必填。调用方应传入工作流实际所属项目；当前后端仅按名称查询，不核对项目归属，响应的 `project_key` 直接使用 Query 值，不能据此确认资源实际归属。本次沿用该行为，不将项目核对混入详情字段补齐。保留现有 `workflow_key`、`workflow_name`、`project_key` 和任务配置结构，不把详情响应改成完整 PUT 的请求模型。

**本次补齐字段**

| 参数名 | 类型 | 描述 |
| --- | --- | --- |
| `template_binding` | TemplateBinding | 已保存的绑定信息；未绑定时可省略 |
| `stages[].id` | string | 稳定阶段 ID；任务 ID 已包含在原 `jobs` 结构中 |
| `updated_by` | string | 已有字段，补齐真实更新人赋值 |

**TemplateBinding 字段**

| 参数名 | 类型 | 描述 |
| --- | --- | --- |
| `enabled` | bool | 是否绑定 |
| `template_id`、`template_name` | string | 绑定模板的 ID 和名称 |
| `base_version` | int | 当前绑定的基准版本 |
| `base_version_id` | string | 基准快照 ID |
| `delta_patches` | []JSONPatchOperation | 相对基准版本的完整增量 |
| `conflict_count` | int | 保存的冲突数；本接口更新成功后清零，不代表已检查最新模板兼容性 |
| `invalid_patches` | []InvalidJSONPatch | 保存的无效增量；本接口更新成功后清空，无此项时可省略，每项包含 `patch` 和 `error` |

绑定工作流的继承字段按基准快照及增量渲染，绑定状态取保存值；由旧更新路径写入的状态仍沿用原有诊断语义。GET 不升级版本、不重新计算对最新模板的冲突、不写回状态；基准渲染失败时返回错误，不省略无效增量后返回看似成功的定义。新绑定更新接口也不进行最新模板比较或迁移。

**返回示例（节选）**

```json
{
  "workflow_key": "pelican-web-publish",
  "workflow_name": "鹈鹕应用发布",
  "project_key": "pelican",
  "template_binding": {
    "enabled": true,
    "template_id": "690000000000000000000001",
    "template_name": "ai-web-publish",
    "base_version": 3,
    "base_version_id": "690000000000000000000003",
    "delta_patches": [],
    "conflict_count": 0
  },
  "stages": [
    {
      "id": "stage-build",
      "name": "构建",
      "jobs": [
        {
          "id": "job-build",
          "name": "build",
          "type": "zadig-build",
          "spec": {
            "service_and_builds_options": [],
            "default_service_and_builds": [],
            "service_and_builds": []
          }
        }
      ]
    }
  ]
}
```

现有任务配置可能包含凭据，本次不全局改变旧详情接口的明文返回语义；调用方须保护响应。新增绑定字段中的增量同样可能含敏感值，不能假定只有任务 `spec` 需要保护。

## 替换工作流绑定增量

**请求**

```text
PUT /openapi/workflows/custom/<name>/template-binding?projectKey=<项目标识>
Content-Type: application/json
```

路径 `name` 与 Query `projectKey` 均必填。按项目和名称联合定位并核对实际归属，后续更新按已定位资源的 ID 保存，不再仅按名称重新查询。Body 的可编辑字段只有 `delta_patches`，不使用调用方提交的项目、模板身份、版本、工作流定义或其他实例字段；未绑定的工作流返回业务错误，不隐式建立绑定。

本节为新增接口的契约。`delta_patches` 必须提供非 null 数组，允许 `[]`；不要求数组非空。

**Query 参数**

| 参数名 | 类型 | 描述 | 必填 |
| --- | --- | --- | --- |
| `projectKey` | string | 工作流实际所属项目标识 | 是 |

**Body 参数**

| 参数名 | 类型 | 描述 | 必填 |
| --- | --- | --- | --- |
| `delta_patches` | []JSONPatchOperation | 相对当前基准版本的完整、有序增量集合；缺失或 `null` 报错 | 是 |

`[]` 清除全部实例增量；遗漏已有增量即移除对应修改。服务端读取并复制当前工作流，只替换增量，以保留描述、通知、审批、禁用状态及其他实例字段和凭据；模板继承字段再由模板加增量生成。服务端维护绑定状态和更新信息，模板身份、基准版本和基准快照 ID 保持不变。

仅在当前基准快照上按顺序应用完整请求增量，不查询最新版本、不升级模板、不迁移增量。任何增量无法应用时返回错误，整次更新不保存；`[]` 按当前基准快照恢复继承字段。管理员发布新模板版本、重排或删除任务，不影响仍绑定旧版本的本次更新。

基准渲染、业务校验和保存使用同一结果。读取基准快照或业务校验失败时返回错误。成功后 `conflict_count` 置为 0、`invalid_patches` 清空，避免保留旧增量的过期诊断；这不表示与最新模板无冲突。最新模板是否可用不影响本次更新。

**Body 示例**

```json
{
  "delta_patches": [
    {
      "op": "add",
      "path": "/stages/0/jobs/0/spec/service_and_builds_options/-",
      "value": {
        "service_name": "pelican-web",
        "service_module": "web",
        "build_name": "pelican-web-build"
      }
    }
  ]
}
```

示例假设无旧增量、目标为第一阶段第一任务，且候选数组已存在。实际路径和内容取自当前详情，不能硬编码下标或覆盖其他候选项。只修改 `service_and_builds_options` 表示增加候选；`default_service_and_builds` 是默认选择，`service_and_builds` 是运行选择，三者不可混用。候选项沿用真实的服务名、组件名和构建名，不使用展示名替代。

**返回：HTTP `200`，`{"message":"success"}`。** 本接口不启动发布，也不改变基准；在基准和其他实例配置未变的前提下，重放相同完整增量得到相同工作流定义。

### JSONPatchOperation

复用现有操作类型和执行器，保留已有增量兼容性，不通过收窄操作枚举来回避输入校验。

| 参数名 | 类型 | 描述 | 必填 |
| --- | --- | --- | --- |
| `op` | string | `add`、`remove`、`replace`、`move`、`copy`、`test` | 是 |
| `path` | string | JSON Pointer 目标路径 | 是 |
| `from` | string | 来源路径，`move`、`copy` 必填；其他操作不要求 | 条件必填 |
| `value` | JSON value | 沿用现有模型：未提供与显式 `null` 均按 null 处理；是否能应用由操作和目标字段决定 | 否 |

沿用现有增量模型，不额外区分未提供 `value` 与显式 `null`。空值保存时可被省略，回读后仍输出 `"value": null`，执行时仍按 null 处理，无需增加字段存在性校验或调整存储模型。

路径处理沿用现有 JSON Patch 执行器，包括 remove 允许目标不存在的行为，不宣称严格实现 RFC 6902。

## 错误返回

沿用现有 Zadig 错误结构和业务码映射。

| HTTP 状态 | 场景 |
| --- | --- |
| `400` | 通过现有参数或业务错误类型返回的错误，例如参数无效、工作流重复、绑定创建失败；由具体错误类型决定 |
| `403` | 身份无效或权限不足；沿用授权响应，可能无 JSON Body |
| `500` | 未包装为业务错误类型的普通错误；现有详情查询直接返回查询错误，工作流不存在也可能为 `500` |

```json
{
  "type": "error",
  "message": "Bad Request",
  "code": 400,
  "description": "delta_patches is required",
  "extra": {}
}
```

现有 `ErrGetWorkflowTemplate`、`ErrFindWorkflow`、`ErrUpsertWorkflow` 等 6000+、7000+ 业务码映射 HTTP `400`，Body 的 `code` 保留业务码；不承诺重复返回 `409`、不存在返回 `404`。

普通错误的响应只包含 `message`、`code`、`description`，不保证包含上述示例中的 `type`、`extra`。新增接口复用现有参数及业务错误类型返回错误，不为已有接口重新统一错误状态或响应结构。

## 实现与验收

以下为验收要求，尚未完成平台联调实测。Swagger 注释与接口实现同步维护，完成验收后再同步正式版本文档。

| 范围 | 验收要点 |
| --- | --- |
| 模板列表 | 空列表、无快照为 0、任意非空分类按原值过滤、仅字符串 true 排除内置模板、条件项目权限、预览不泄露凭据；GET 无写操作 |
| 工作流创建 | name/project 必填，display_name 可省略；template_id 的空值和非法格式处理沿用现有行为；不提交 stages 可绑定创建；省略版本、正数及 0 选版符合契约，负数保留既有语义；收尾不升级；仍有绑定且无运行任务 |
| 工作流详情 | 基准渲染、稳定 ID、真实更新人、绑定回读、无写入 |
| 绑定更新 | 缺失 projectKey 拒绝、项目与名称联合定位、跨项目同名不误更新；未绑定拒绝、缺失/null/空数组、完整增量替换、空数组恢复基准、基准版本和快照 ID 不变；新模板重排或删除任务不影响本次更新，当前基准无法应用任一增量时拒绝整次更新；校验与保存一致、已有配置与凭据保留 |
| 增量校验 | value 未提供与显式 null 均沿用现有空值执行及保存回读语义；操作、来源和路径的处理沿用现有执行器；已有六种操作兼容 |

原完整 PUT 的实例字段替换语义保持不变。统一模板渲染会修正旧绑定请求中“直传继承字段与增量不一致”的行为：既有绑定创建/更新调用方需将修改表达在增量里，不能依赖另一份 `stages` 覆盖模板。未绑定工作流不切换到模板渲染。

现有名称预检查按全局 name 查询，但数据库唯一索引是 project + name；本文不扩展为全局唯一性改造。现有更新没有条件版本比较，新入口同样不承诺并发编辑安全，不新增锁或 ETag。

已有详情和完整更新接口的权限修复独立处理，不纳入本需求，也不作为本次交付的新增前置条件。新绑定更新接口仍按项目和名称联合定位，并复用既有绑定编辑规则授权。
