# 代理配置

本文介绍 PR #5017 收口后的系统代理配置 API，以及构建步骤的代理开关。代理配置接口只扩展 HTTPS、SOCKS5 的协议支持；构建 API 新增可选的 `docker_build_step.enable_proxy` 字段。

## 接口变更概览

| 场景 | 本次变更 |
| --- | --- |
| 代码源运行时代理 | 在原有 HTTP、SOCKS5 处理之外增加 HTTPS 类型；`GET /api/aslan/system/proxy/config` 返回字段保持不变 |
| Git HTTP/HTTPS 拉取 | 原有代码源代理开关开启时，增加使用 HTTPS、SOCKS5 代理 |
| Docker build 代理参数 | 增加 `docker_build_step.enable_proxy` 构建步骤开关；HTTP、HTTPS 代理可注入 build 参数，SOCKS5 仍不自动传入 build 参数 |

代理配置的创建、更新、删除、查询和连接测试沿用现有接口。权限、密码保存与掩码处理、URL 拼接、软件包下载、SSH 转发、DinD 和配置删除后的运行时处理均保留原有实现。

## 调用方式

使用 Zadig 系统访问地址，在 HTTP Header 中传入 API Token。除列出代理配置外，本文代理接口均要求系统管理员权限。

**接口权限说明**

以下为对外调用的现有权限规则，PR #5017 未修改权限逻辑。API Token 使用所属用户的权限，项目管理员身份不等同于系统管理员身份。

| 请求 | 权限要求 |
| --- | --- |
| `GET /api/aslan/system/proxyManage` | 沿用原有身份鉴权；接口处理函数未启用系统管理员角色校验，本次未修改 |
| `GET /api/aslan/system/proxyManage/:id` | 系统管理员 |
| `POST /api/aslan/system/proxyManage` | 系统管理员 |
| `PUT /api/aslan/system/proxyManage/:id` | 系统管理员 |
| `DELETE /api/aslan/system/proxyManage/:id` | 系统管理员 |
| `POST /api/aslan/system/proxyManage/connectionTest` | 系统管理员 |
| `GET /api/aslan/system/proxy/config` | 系统管理员 |

```bash
curl -H 'Authorization: Bearer your-token' \
  'https://yours.zadig.com/api/aslan/system/proxyManage'
```

创建、更新和连接测试需设置 `Content-Type: application/json`。

## 创建代理配置

**请求**

```text
POST /api/aslan/system/proxyManage
```

**body 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `type` | 代理类型：`http`、`https`、`socks5`；`no` 表示不使用代理 | string | 是 |
| `address` | 代理服务器域名或 IP，不带协议、端口或路径；IPv6 地址需带方括号，例如 `[2001:db8::1]` | string | 使用代理时必填 |
| `port` | 代理服务器端口 | int | 使用代理时必填 |
| `need_password` | 是否启用代理认证，默认 `false` | bool | 否 |
| `username` | 代理认证用户名，按原始值传入，无需 URL 编码 | string | 启用认证时必填 |
| `password` | 代理认证密码，按原始值传入，无需 URL 编码 | string | 启用认证时必填 |
| `usage` | 历史用途字段，沿用 `default` | string | 否 |
| `enable_repo_proxy` | 历史字段，当前仍用于代码源系统代理及 Docker build 参数的启用判断，默认 `false` | bool | 否 |
| `enable_application_proxy` | 历史应用代理开关，用于 Reaper 安装脚本、消息通知等现有调用方，默认 `false` | bool | 否 |

表中“必填”表示构造有效代理配置所需的字段。现有接口使用 JSON 类型绑定，未增加字段必填、代理类型枚举或端口范围校验。

**body 参数示例**

```json
{
  "type": "https",
  "address": "proxy.example.com",
  "port": 8443,
  "need_password": true,
  "username": "proxy-user",
  "password": "example-password",
  "usage": "default",
  "enable_repo_proxy": true,
  "enable_application_proxy": true
}
```

**成功返回示例**

HTTP 状态码为 `200`。创建接口不返回新配置的 ID，可通过列表接口查询。

```json
{
  "message": "success"
}
```

## 列出代理配置

**请求**

```text
GET /api/aslan/system/proxyManage
```

**Query 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `encryptedKey` | 沿用现有密码加密查询协议。未传时，非空密码返回掩码 `********`；传入有效密钥时返回加密后的密码 | string | 否 |

**成功返回说明**

返回代理对象数组，无配置时返回 `[]`，按创建时间倒序排列。运行时使用列表中的第一条配置，不合并多条配置。

每个对象的返回字段如下：

| 参数名 | 说明 | 类型 |
| --- | --- | --- |
| `id` | 代理配置 ID | string |
| `type` | 代理类型：`http`、`https`、`socks5`、`no` | string |
| `address` | 代理服务器域名或 IP | string |
| `port` | 代理服务器端口 | int |
| `need_password` | 是否启用代理认证 | bool |
| `username` | 代理认证用户名 | string |
| `password` | 代理认证密码；未传 `encryptedKey` 时，非空密码返回 `********` | string |
| `usage` | 历史用途字段 | string |
| `enable_repo_proxy` | 代码源系统代理及 Docker build 参数的启用开关 | bool |
| `enable_application_proxy` | 历史应用代理开关，用于 Reaper 安装脚本、消息通知等现有调用方 | bool |
| `create_time` | 创建时间，Unix 时间戳，单位为秒 | int64 |
| `update_time` | 更新时间，Unix 时间戳，单位为秒 | int64 |
| `update_by` | 最后更新人 | string |

**成功返回示例**

```json
[
  {
    "id": "507f1f77bcf86cd799439011",
    "type": "https",
    "address": "proxy.example.com",
    "port": 8443,
    "need_password": true,
    "username": "proxy-user",
    "password": "********",
    "usage": "default",
    "enable_repo_proxy": true,
    "enable_application_proxy": true,
    "create_time": 1791507600,
    "update_time": 1791507600,
    "update_by": "admin"
  }
]
```

## 获取指定代理配置

**请求**

```text
GET /api/aslan/system/proxyManage/:id
```

**路径参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | 代理配置 ID，24 位十六进制字符串 | string | 是 |

**Query 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `encryptedKey` | 沿用现有密码加密查询协议。未传时，非空密码返回掩码 `********`；传入有效密钥时返回加密后的密码 | string | 否 |

**成功返回说明**

返回单个代理对象，HTTP 状态码为 `200`。

| 参数名 | 说明 | 类型 |
| --- | --- | --- |
| `id` | 代理配置 ID | string |
| `type` | 代理类型：`http`、`https`、`socks5`、`no` | string |
| `address` | 代理服务器域名或 IP | string |
| `port` | 代理服务器端口 | int |
| `need_password` | 是否启用代理认证 | bool |
| `username` | 代理认证用户名 | string |
| `password` | 代理认证密码；未传 `encryptedKey` 时，非空密码返回 `********` | string |
| `usage` | 历史用途字段 | string |
| `enable_repo_proxy` | 代码源系统代理及 Docker build 参数的启用开关 | bool |
| `enable_application_proxy` | 历史应用代理开关，用于 Reaper 安装脚本、消息通知等现有调用方 | bool |
| `create_time` | 创建时间，Unix 时间戳，单位为秒 | int64 |
| `update_time` | 更新时间，Unix 时间戳，单位为秒 | int64 |
| `update_by` | 最后更新人 | string |

**成功返回示例**

```json
{
  "id": "507f1f77bcf86cd799439011",
  "type": "https",
  "address": "proxy.example.com",
  "port": 8443,
  "need_password": true,
  "username": "proxy-user",
  "password": "********",
  "usage": "default",
  "enable_repo_proxy": true,
  "enable_application_proxy": true,
  "create_time": 1791507600,
  "update_time": 1791507600,
  "update_by": "admin"
}
```

## 更新指定代理配置

**请求**

```text
PUT /api/aslan/system/proxyManage/:id
```

**路径参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | 代理配置 ID，24 位十六进制字符串 | string | 是 |

**body 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `type` | 代理类型：`http`、`https`、`socks5`；`no` 表示不使用代理 | string | 是 |
| `address` | 代理服务器域名或 IP，不带协议、端口或路径；IPv6 地址需带方括号，例如 `[2001:db8::1]` | string | 使用代理时必填 |
| `port` | 代理服务器端口 | int | 使用代理时必填 |
| `need_password` | 是否启用代理认证，未传时更新为 `false` | bool | 否 |
| `username` | 代理认证用户名，按原始值传入，无需 URL 编码 | string | 启用认证时必填 |
| `password` | 代理认证密码，原文传入；传 `********` 时保留已有密码 | string | 启用认证时必填 |
| `usage` | 历史用途字段，需保留原值 | string | 否 |
| `enable_repo_proxy` | 代码源系统代理及 Docker build 参数的启用开关，未传时更新为 `false` | bool | 否 |
| `enable_application_proxy` | 历史应用代理开关，用于 Reaper 安装脚本、消息通知等现有调用方；未传时更新为 `false` | bool | 否 |

> 更新方式：此接口按完整配置更新，不是部分更新。需传回需要保留的配置字段；遗漏布尔字段会更新为 `false`，遗漏字符串字段会更新为空字符串。`enable_repo_proxy`、`enable_application_proxy` 仍参与现有运行逻辑，也需保留原值。

沿用现有密码处理规则：`password` 为 `********` 时保留已保存密码；修改密码时传入新密码原文。请求中的 `id` 不是更新目标，更新目标以路径参数为准。

**body 参数示例**

以下示例保留已有密码：

```json
{
  "type": "https",
  "address": "proxy.example.com",
  "port": 8443,
  "need_password": true,
  "username": "proxy-user",
  "password": "********",
  "usage": "default",
  "enable_repo_proxy": true,
  "enable_application_proxy": true
}
```

**成功返回示例**

```json
{
  "message": "success"
}
```

## 删除指定代理配置

**请求**

```text
DELETE /api/aslan/system/proxyManage/:id
```

**路径参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `id` | 代理配置 ID，24 位十六进制字符串 | string | 是 |

无 body 参数。

删除后按原有流程刷新 Aslan 代码源代理配置。本次没有新增 DinD 同步，也没有修改删除最后一条配置时的运行时处理。

**成功返回示例**

```json
{
  "message": "success"
}
```

## 测试代理连接

**请求**

```text
POST /api/aslan/system/proxyManage/connectionTest
```

**body 参数说明**

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `type` | 待测试代理类型：`http`、`https`、`socks5` | string | 是 |
| `address` | 代理服务器域名或 IP，不带协议、端口或路径；IPv6 地址需带方括号，例如 `[2001:db8::1]` | string | 是 |
| `port` | 代理服务器端口 | int | 是 |
| `need_password` | 是否启用代理认证，默认 `false` | bool | 否 |
| `username` | 代理认证用户名，按原始值传入，无需 URL 编码 | string | 启用认证时必填 |
| `password` | 代理认证密码，原文传入；传 `********` 时使用指定配置的已保存密码 | string | 启用认证时必填 |
| `id` | 已保存的代理配置 ID，24 位十六进制字符串 | string | `password` 为 `********` 时必填 |

测试不会保存配置，也不依赖 `enable_repo_proxy` 或 `enable_application_proxy` 两个用途开关。

测试已保存配置且 `password` 为 `********` 时，需同时传入该配置的 `id`，后端使用已保存密码完成测试。

**body 参数示例**

```json
{
  "type": "socks5",
  "address": "proxy.example.com",
  "port": 1080,
  "need_password": true,
  "username": "proxy-user",
  "password": "example-password"
}
```

**成功返回示例**

```json
{
  "message": "success"
}
```

测试由 Aslan 经代理访问 `https://www.baidu.com`，请求超时为 5 秒，目标返回 `200` 时成功。测试成功仅代表该访问路径可用，不代表 Git SSH、安装脚本或各集群 DinD 镜像拉取已验证成功。

## 获取代码源运行时代理地址

**请求**

```text
GET /api/aslan/system/proxy/config
```

此接口直接返回 Aslan 当前使用的代码源代理地址。配置刷新沿用原有流程，已有配置的 `enable_repo_proxy=false` 时会清空地址；没有配置或切换为 `no` 时保留旧处理方式，可能仍返回此前的运行时地址。

**成功返回说明**

| 参数名 | 说明 | 类型 |
| --- | --- | --- |
| `HTTPAddr` | `http`、`https`、`socks5` 类型对应的代理 URL | string |
| `HTTPSAddr` | `http` 或 `https` 类型对应的代理 URL；`socks5` 类型为空字符串 | string |
| `Socks5Addr` | `socks5` 类型对应的代理 URL；其他类型为空字符串 | string |
| `NoProxy` | 现有保留字段，当前为空字符串 | string |

**成功返回示例**

以下为无需认证的 HTTPS 代理配置：

```json
{
  "HTTPAddr": "https://proxy.example.com:8443",
  "HTTPSAddr": "https://proxy.example.com:8443",
  "Socks5Addr": "",
  "NoProxy": ""
}
```

## 代理生效范围

| 场景 | 开启条件 | 本次支持范围 |
| --- | --- | --- |
| Git HTTP/HTTPS 拉取 | `enable_repo_proxy=true`，且对应代码源开启代理 | 在原有 `http` 基础上增加 `https`、`socks5` |
| Aslan 代码源运行时代理 | `enable_repo_proxy=true` | 在原有 `http`、`socks5` 基础上增加 `https` |
| Docker build 预定义代理参数 | 全局 `enable_repo_proxy=true`，且构建步骤 `enable_proxy` 未设置或为 `true` | 在原有 `http` 基础上增加 `https`，不自动传入 SOCKS5 |

本次没有新增软件包下载代理、SSH 转发器、DinD 独立开关或自定义绕过列表。原有安装脚本、SSH 和镜像拉取路径继续使用原实现。

### 构建步骤代理开关

这个开关和页面中的 BuildKit 开关一样，属于通用的 `DockerBuild` 配置，不是 OpenAPI 专属字段。普通构建接口使用 `post_build.docker_build.enable_proxy`，OpenAPI 则使用 `docker_build_step.enable_proxy`。两种入口最终写入同一个构建配置。

通过接口配置时，相关请求包括：

| 请求 | 说明 |
| --- | --- |
| `POST /api/build/build` | 普通接口新建构建 |
| `PUT /api/build/build` | 普通接口更新构建 |
| `POST /api/template/build` | 普通接口新建构建模板 |
| `PUT /api/template/build/<构建模板 ID>` | 普通接口更新构建模板 |
| `POST /openapi/build?projectKey=<项目标识>` | 新建构建 |
| `PUT /openapi/build?projectKey=<项目标识>` | 更新构建 |
| `GET /openapi/build/<构建名称>/detail?projectKey=<项目标识>` | 获取构建详情 |
| `POST /openapi/templates/builds` | 新建构建模板 |
| `PUT /openapi/templates/builds/<构建模板 ID>` | 更新构建模板 |
| `GET /openapi/templates/builds/<构建模板 ID>` | 获取构建模板详情 |

字段示例：

```json
{
  "docker_build_step": {
    "dockerfile_source": "local",
    "build_context_dir": "$REPONAME_0",
    "dockerfile_directory": "$REPONAME_0/Dockerfile",
    "enable_proxy": false
  }
}
```

兼容规则如下：

- 未传入或传入 `null`：沿用旧实现，由全局 `enable_repo_proxy` 决定是否注入代理。
- 传入 `true`：仍要求全局 `enable_repo_proxy=true`，然后按代理类型处理。
- 传入 `false`：只关闭当前镜像构建步骤的代理注入，不影响代码源代理或其他构建步骤。
- 代理类型为 `http` 或 `https` 时，注入 `http_proxy` 和 `https_proxy` 两个 Docker build-arg。
- 代理类型为 `socks5` 时，不把 SOCKS5 地址作为 Docker build-arg 传入，以保持旧用户构建兼容性。

该字段只控制 Docker build 的代理 build-arg，不控制 Git 拉取、软件包安装、Docker 登录/推送或 DinD 镜像拉取。已有请求不传该字段即可保持旧行为。

构建更新接口按完整配置更新。如果已有构建明确设置了 `enable_proxy=false`，调用方更新时需要继续传入 `false`；旧客户端省略该字段后，该字段会恢复为未设置，并重新按全局 `enable_repo_proxy` 处理。

软件包文件下载未增加系统代理配置读取。Reaper 安装脚本仍按 `enable_application_proxy` 开关导出 `http_proxy`、`https_proxy`，jobexecutor 的安装步骤保留原有处理。

SOCKS5 使用注意：Git HTTP/HTTPS 使用 `socks5://` 时由本地解析目标域名，运行环境需具备相应 DNS 解析能力。API 的 `type` 使用 `socks5`，不使用 `socks5h`。HTTPS 代理未提供自定义 CA 字段，自签证书需由运行环境的信任链支持。

用户名和密码按原文传入，密码掩码与加密查询规则沿用旧接口。代理 URL 也沿用旧拼接方式，本次没有新增认证信息特殊字符编码。

## 错误返回

沿用现有错误格式。无权限时返回 `403`；参数解析错误返回 `400`。代理业务错误返回 HTTP `400`，具体业务错误码如下：

| 错误码 | 说明 |
| --- | --- |
| `6801` | 获取代理失败 |
| `6802` | 创建代理失败 |
| `6803` | 更新代理失败 |
| `6804` | 列出代理失败 |
| `6805` | 删除代理失败 |
| `6806` | 代理连接测试失败 |

**错误返回示例**

```json
{
  "type": "error",
  "message": "代理连接测试失败",
  "code": 6806,
  "description": "具体失败原因",
  "extra": null
}
```
