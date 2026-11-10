# AI 能力后端接入契约

系统权限资源为 `AIManagement`，显示名称为“AI 能力”。两个权限统一覆盖模型管理、Zadig AI 配置、代码审查配置、发布专员配置和 Skill 管理。

| 权限 | 授权响应字段 | 能力 |
| --- | --- | --- |
| `get_ai_management` | `system_actions.ai_management.View` | 读取上述配置 |
| `edit_ai_management` | `system_actions.ai_management.Edit` | 读取及编辑上述配置，新增、删除模型及验证模型连接 |

系统管理员直接允许；其他用户默认无权限，全局只读角色不自动获得权限。编辑权限隐含读取能力，前端显示入口时应判断 `View || Edit`，保存及模型管理操作判断 `Edit`。权限不足返回标准 403 错误。Zadig AI 配置仍受企业版许可证限制。

## 发布专员配置

- 读取：`GET /api/aslan/system/ai/release-specialist`
- 保存：`PUT /api/aslan/system/ai/release-specialist`

GET 响应与 PUT 请求均为：

```json
{"llm_integration_id":"已有模型的 ID"}
```

PUT 成功沿用标准响应 `{"message":"success"}`。模型不存在或 ID 无效返回标准参数错误，配置读写失败返回标准内部错误。

使用 `GET /api/aslan/system/llm/integrations` 填充单选模型列表，响应为 `[{"id":"...","name":"..."}]`，该接口仅要求登录，不返回凭据或服务地址。保留模型存在性检查接口的现有行为。

尚未配置时 GET 返回 `{"llm_integration_id":""}`。保存空 ID 表示使用默认模型。发布专员的规则编译和任务执行均使用该系统配置，不增加工作流级模型字段。

已选模型不存在或不可用时，发布专员返回错误，不自动切换到默认模型。配置变更对后续模型调用生效，现有有效规则计划缓存继续复用。系统配置使用 `ai_release_specialist_config` 字段局部持久化，无需迁移历史数据。

## 其他配置接口

| 接口 | 权限 |
| --- | --- |
| `GET /api/aslan/system/llm/integration`、`GET /api/aslan/system/llm/integration/:id` | 查看或编辑 |
| `POST /api/aslan/system/llm/integration`、`PUT/DELETE /api/aslan/system/llm/integration/:id` | 编辑 |
| `POST /api/aslan/system/llm/integration/validate` | 编辑 |
| `GET /api/aslan/system/ai/assistant`、`GET /api/aslan/system/ai/review` | 查看或编辑 |
| `PUT /api/aslan/system/ai/assistant`、`PUT /api/aslan/system/ai/review` | 编辑 |
| `GET /api/halman/skills` | 查看或编辑 |
| `PUT /api/halman/skills/:skill_id` | 编辑 |

本次不包含前端页面改动。
