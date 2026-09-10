# Helm 服务 OpenAPI

本文介绍 Helm 服务配置的新建、详情、更新和删除，以及环境服务 Values 查询、来源配置、更新预览和更新 OpenAPI。

::: tip
接口返回的 Values 会对密码、Token、密钥、凭据等敏感字段进行脱敏，脱敏后的值固定为 `********`。字段名称匹配不区分大小写，并兼容下划线、连字符和驼峰命名。`existingSecret`、`pullSecret`、`secretName` 等 Secret 引用名保留原值。更新请求中的 Values 在写入操作日志前使用相同规则脱敏。
:::

## 权限说明

| 接口 | 所需权限 |
| --- | --- |
| 测试服务配置新建 | 服务新建权限 |
| 生产服务配置新建 | 生产服务新建权限 |
| 测试服务配置详情 | 服务查看权限 |
| 生产服务配置详情 | 生产服务查看权限 |
| 测试服务配置更新 | 服务编辑权限 |
| 生产服务配置更新 | 生产服务编辑权限 |
| 测试服务配置删除 | 服务删除权限 |
| 生产服务配置删除 | 生产服务删除权限 |
| 测试环境 Values 查询 | 测试环境查看权限 |
| 生产环境 Values 查询 | 生产环境查看权限 |
| 测试环境 Values 来源查询 | 测试环境查看权限 |
| 生产环境 Values 来源查询 | 生产环境查看权限 |
| 测试环境 Values 来源配置 | 测试环境配置权限 |
| 生产环境 Values 来源配置 | 生产环境配置权限 |
| 测试环境 Values 更新预览 | 测试环境配置权限 |
| 生产环境 Values 更新预览 | 生产环境配置权限 |
| 测试环境 Values 更新 | 测试环境配置权限 |
| 生产环境 Values 更新 | 生产环境配置权限 |

访问生产服务或生产环境时，还需要有效的 Zadig 专业版许可证。

## 新建 Helm 服务配置

### 从代码库或 Chart 仓库新建

支持从已接入代码源、公开代码库和 Chart 仓库新建 Helm 服务配置。

**请求**

```
POST /openapi/service/helm/load?projectKey=<项目标识>
```

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `source` | string | 服务来源，支持 `repo`、`gerrit`、`gitee`、`gitee-enterprise`、`publicRepo`、`chartRepo` | 是 |
| `production` | bool | 是否新建生产服务，默认值为 `false` | 否 |
| `createFrom` | object | 服务来源配置，不同 `source` 对应字段见下方说明 | 是 |

**已接入代码源的 createFrom 参数说明**

适用于 `repo`、`gerrit`、`gitee` 和 `gitee-enterprise`。

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `codehostName` | string | 代码源名称 | 是 |
| `owner` | string | 仓库拥有者或组织名 | 是 |
| `namespace` | string | 仓库命名空间，为空时使用 `owner` | 否 |
| `repo` | string | 代码库名称 | 是 |
| `branch` | string | 分支名称 | 是 |
| `paths` | []string | Chart 所在路径列表 | 是 |

**publicRepo 的 createFrom 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `repoLink` | string | 公开代码库地址 | 是 |
| `paths` | []string | Chart 所在路径列表 | 是 |

**chartRepo 的 createFrom 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `chartRepoName` | string | Chart 仓库名称 | 是 |
| `chartName` | string | Chart 名称 | 是 |
| `chartVersion` | string | Chart 版本约束，为空时由 Helm 解析可用版本 | 否 |

**Body 参数示例**

从已接入代码源新建：

::: details

```json
{
  "source": "repo",
  "production": false,
  "createFrom": {
    "codehostName": "github-demo",
    "owner": "koderover",
    "repo": "helm-charts",
    "branch": "main",
    "paths": [
      "charts/backend"
    ]
  }
}
```

:::

从 Chart 仓库新建：

::: details

```json
{
  "source": "chartRepo",
  "production": false,
  "createFrom": {
    "chartRepoName": "stable",
    "chartName": "backend",
    "chartVersion": "1.2.0"
  }
}
```

:::

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `successServices` | []string | 新建成功的服务名称列表 |
| `failedServices` | []FailedService | 新建失败的服务列表 |

**FailedService 参数说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `path` | string | 新建失败的 Chart 路径 |
| `error` | string | 失败原因 |

**返回示例**

```json
{
  "successServices": [
    "backend"
  ],
  "failedServices": []
}
```

### 使用模板新建

使用模板库中的 Helm 模板新建服务配置，可同时设置模板变量和 Values。开启自动同步后，模板更新时会同步更新服务配置。

**请求**

```
POST /openapi/service/template/load/helm?projectKey=<项目标识>
```

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `service_name` | string | 服务名称 | 是 |
| `production` | bool | 是否新建生产服务，默认值为 `false` | 否 |
| `template_name` | string | 模板库中的 Helm 模板名称 | 是 |
| `values_yaml` | string | 在模板默认 Values 基础上覆盖的 Values YAML | 否 |
| `variables` | []KeyValue | 模板变量 | 否 |
| `auto_sync` | bool | 是否自动同步模板更新，默认值为 `false` | 否 |

**KeyValue 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `key` | string | 变量名称 | 是 |
| `value` | any | 变量值 | 是 |

**Body 参数示例**

::: details

```json
{
  "service_name": "backend",
  "production": false,
  "template_name": "general-chart",
  "values_yaml": "replicaCount: 2\nimage:\n  tag: v1.2.0\n",
  "variables": [
    {
      "key": "port",
      "value": 8080
    }
  ],
  "auto_sync": true
}
```

:::

**返回**

成功时返回 HTTP 状态码 `200`。

## 获取 Helm 服务配置详情

### 测试服务

**请求**

```
GET /openapi/service/helm/<服务名称>?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `serviceName` | string | 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `service_name` | string | 服务名称 |
| `type` | string | 服务类型，固定值 `helm` |
| `source` | string | 服务来源，支持 `chartTemplate`、`chartRepo`、`repo`、`publicRepo`、`gerrit`、`gitee`、`gitee-enterprise` |
| `source_detail` | SourceDetail | 服务来源详情，字段随 `source` 不同而变化 |
| `revision` | int | 当前服务版本 |
| `chart_name` | string | Chart 名称 |
| `chart_version` | string | Chart 版本 |
| `values_yaml` | string | 服务配置的 Values YAML，敏感字段返回 `********` |
| `release_naming` | string | Helm Release 命名规则 |
| `containers` | []Container | 服务组件列表 |
| `created_by` | string | 当前服务版本创建者 |
| `created_time` | int | 当前服务版本创建时间，Unix 时间戳格式 |

`source` 为 `chartTemplate` 时，通过 `source_detail.customized` 区分是否经过自定义编辑。GitHub、GitLab 和 `other` 类型的已接入代码源统一返回 `repo`。Gerrit 来源的 `owner`、`namespace` 返回已保存的仓库信息，历史配置未保存时为空字符串。

**SourceDetail 参数说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `template_name` | string | 模板名称，`source` 为 `chartTemplate` 时返回 |
| `customized` | bool | 是否已自定义编辑，`source` 为 `chartTemplate` 时返回 |
| `auto_sync` | bool | 是否自动同步模板更新，`source` 为 `chartTemplate` 时返回 |
| `codehost_name` | string | 代码源名称，`source` 为已接入代码源时返回 |
| `owner` | string | 仓库拥有者或组织名，`source` 为已接入代码源时返回 |
| `namespace` | string | 仓库命名空间，`source` 为已接入代码源时返回 |
| `repo` | string | 代码库名称，`source` 为已接入代码源时返回 |
| `branch` | string | 分支名称，`source` 为已接入代码源时返回 |
| `path` | string | Chart 所在路径，`source` 为代码库来源时返回 |
| `repo_url` | string | 公开代码库地址，`source` 为 `publicRepo` 时返回 |
| `chart_repo_name` | string | Chart 仓库名称，`source` 为 `chartRepo` 时返回 |

**Container 参数说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `name` | string | 服务组件名称 |
| `image` | string | 服务组件镜像 |
| `image_name` | string | 服务组件镜像名称 |

**返回示例**

::: details

```json
{
  "service_name": "backend",
  "type": "helm",
  "source": "chartTemplate",
  "source_detail": {
    "template_name": "general-chart",
    "customized": false,
    "auto_sync": true
  },
  "revision": 3,
  "chart_name": "backend",
  "chart_version": "1.2.0",
  "values_yaml": "replicaCount: 2\nimage:\n  repository: koderover/backend\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "release_naming": "$Namespace$-$Service$",
  "containers": [
    {
      "name": "backend",
      "image": "koderover/backend:v1.2.0",
      "image_name": "backend"
    }
  ],
  "created_by": "admin",
  "created_time": 1788861600
}
```

:::

### 生产服务

**请求**

```
GET /openapi/service/helm/production/<服务名称>?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `serviceName` | string | 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `service_name` | string | 服务名称 |
| `type` | string | 服务类型，固定值 `helm` |
| `source` | string | 服务来源，支持 `chartTemplate`、`chartRepo`、`repo`、`publicRepo`、`gerrit`、`gitee`、`gitee-enterprise` |
| `source_detail` | SourceDetail | 服务来源详情，字段定义与测试服务接口一致 |
| `revision` | int | 当前服务版本 |
| `chart_name` | string | Chart 名称 |
| `chart_version` | string | Chart 版本 |
| `values_yaml` | string | 服务配置的 Values YAML，敏感字段返回 `********` |
| `release_naming` | string | Helm Release 命名规则 |
| `containers` | []Container | 服务组件列表 |
| `created_by` | string | 当前服务版本创建者 |
| `created_time` | int | 当前服务版本创建时间，Unix 时间戳格式 |

**Container 参数说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `name` | string | 服务组件名称 |
| `image` | string | 服务组件镜像 |
| `image_name` | string | 服务组件镜像名称 |

**返回示例**

::: details

```json
{
  "service_name": "backend",
  "type": "helm",
  "source": "repo",
  "source_detail": {
    "codehost_name": "gitlab",
    "owner": "koderover",
    "namespace": "koderover",
    "repo": "helm-charts",
    "branch": "main",
    "path": "charts/backend"
  },
  "revision": 3,
  "chart_name": "backend",
  "chart_version": "1.2.0",
  "values_yaml": "replicaCount: 2\nimage:\n  repository: koderover/backend\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "release_naming": "$Namespace$-$Service$",
  "containers": [
    {
      "name": "backend",
      "image": "koderover/backend:v1.2.0",
      "image_name": "backend"
    }
  ],
  "created_by": "admin",
  "created_time": 1788861600
}
```

:::

## 更新 Helm 服务配置

更新服务配置的 `values.yaml`，内容发生变化时生成新的服务版本。请求中的 `expected_revision` 必须与当前版本一致，避免并发更新覆盖。默认不会更新已经部署到环境中的服务；项目开启服务自动部署后，会异步更新使用该服务的测试环境。

::: warning
仅支持更新通过模板库创建的 Helm 服务配置。本接口只更新 `values.yaml`，不支持编辑 Chart 中的其他文件。`values_yaml` 必须传入完整且合法的 YAML 内容。在敏感字段中传入 `********` 表示保留该字段的当前值；当前版本不存在对应字段时，不能使用 `********`，必须传入真实值。
:::

### 测试服务

**请求**

```
PUT /openapi/service/helm/<服务名称>?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `serviceName` | string | 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `expected_revision` | int | 期望更新的服务版本，必须与当前版本一致 | 是 |
| `values_yaml` | string | 完整的 Values YAML 内容 | 是 |

**Body 参数示例**

::: details

```json
{
  "expected_revision": 3,
  "values_yaml": "replicaCount: 3\nimage:\n  repository: koderover/backend\n  tag: v1.3.0\ndatabase:\n  password: new-password\n"
}
```

:::

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `revision` | int | 更新后的服务版本 |

**返回示例**

```json
{
  "revision": 4
}
```

### 生产服务

**请求**

```
PUT /openapi/service/helm/production/<服务名称>?projectKey=<项目标识>
```

路径参数、Query 参数、Body 参数和返回参数与测试服务接口一致。

**返回示例**

```json
{
  "revision": 4
}
```

## 删除 Helm 服务配置

删除项目中的 Helm 服务配置。该操作不会删除模板库中的 Helm 模板。

### 测试服务

**请求**

```
DELETE /openapi/service/helm/<服务名称>?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `serviceName` | string | 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回**

```json
{
  "message": "success"
}
```

### 生产服务

**请求**

```
DELETE /openapi/service/helm/production/<服务名称>?projectKey=<项目标识>
```

路径参数、Query 参数和返回参数与测试服务接口一致。

## 获取环境服务 Values

获取环境保存的服务 Values 配置，以及根据 Zadig 当前配置计算的完整 Values。完整 Values 与更新预览的当前 Values 使用相同的计算规则，不读取集群中 Helm Release 的实时 Values。

### 测试环境

**请求**

```
GET /openapi/environments/helm/<环境标识>/services/<服务名称>/values?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `release_name` | string | 按环境中当前服务版本的命名规则计算的 Helm Release 名称 |
| `revision` | int | 当前 Values 配置版本 |
| `values_yaml` | string | 当前环境覆盖 Values YAML，敏感字段返回 `********` |
| `effective_values_yaml` | string | 根据 Zadig 保存的服务配置 Values、环境全局 Values、服务覆盖 Values、键值覆盖及镜像配置合并计算的完整 Values YAML，敏感字段返回 `********` |
| `override_kvs` | []KeyValue | 当前键值覆盖配置，敏感字段值返回 `********` |

**返回示例**

::: details

```json
{
  "release_name": "demo-backend-dev",
  "revision": 3,
  "values_yaml": "replicaCount: 2\ndatabase:\n  password: '********'\n",
  "effective_values_yaml": "replicaCount: 2\nimage:\n  repository: koderover/backend\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "override_kvs": [
    {
      "key": "resources.limits.cpu",
      "value": "1000m"
    }
  ]
}
```

:::

### 生产环境

**请求**

```
GET /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `release_name` | string | 按环境中当前服务版本的命名规则计算的 Helm Release 名称 |
| `revision` | int | 当前 Values 配置版本 |
| `values_yaml` | string | 当前环境覆盖 Values YAML，敏感字段返回 `********` |
| `effective_values_yaml` | string | 根据 Zadig 保存的服务配置 Values、环境全局 Values、服务覆盖 Values、键值覆盖及镜像配置合并计算的完整 Values YAML，敏感字段返回 `********` |
| `override_kvs` | []KeyValue | 当前键值覆盖配置，敏感字段值返回 `********` |

**返回示例**

::: details

```json
{
  "release_name": "demo-backend-prod",
  "revision": 3,
  "values_yaml": "replicaCount: 3\ndatabase:\n  password: '********'\n",
  "effective_values_yaml": "replicaCount: 3\nimage:\n  repository: koderover/backend\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "override_kvs": []
}
```

:::

## 管理环境服务 Values 来源

为环境服务配置 Git Values 来源和自动同步。配置或清除 Values 来源接口本身不会立即更新环境 Values，也不会立即触发 Helm Release 更新。

### 获取 Values 来源

#### 测试环境

**请求**

```
GET /openapi/environments/helm/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `configured` | bool | 是否已配置 Git Values 来源 |
| `codehost_name` | string | 代码源名称，未配置时不返回 |
| `namespace` | string | 代码库所属命名空间，未配置时不返回 |
| `repo` | string | 代码库名称，未配置时不返回 |
| `branch` | string | 分支名称，未配置时不返回 |
| `value_path` | string | Values 文件路径，未配置时不返回 |
| `auto_sync` | bool | 是否自动同步代码库中的 Values，未配置时不返回 |

**返回示例**

::: details

```json
{
  "configured": true,
  "codehost_name": "gitlab",
  "namespace": "demo",
  "repo": "backend-config",
  "branch": "main",
  "value_path": "helm/dev-values.yaml",
  "auto_sync": true
}
```

:::

#### 生产环境

**请求**

```
GET /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

路径参数、Query 参数和返回参数与测试环境接口一致。

### 配置 Values 来源

配置成功后，调用环境服务 Values 更新接口并将 `sync_values_from_source` 设置为 `true`，可从该来源读取 Values 并更新 Helm Release。开启自动同步后，代码库中的 Values 发生变化时将自动同步到环境。

#### 测试环境

**请求**

```
PUT /openapi/environments/helm/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `codehost_name` | string | 系统级或当前项目下的代码源名称，可用范围内存在同名代码源时返回参数错误 | 是 |
| `namespace` | string | 代码库所属命名空间 | 是 |
| `repo` | string | 代码库名称 | 是 |
| `branch` | string | 分支名称 | 是 |
| `value_path` | string | Values 文件路径 | 是 |
| `auto_sync` | bool | 是否自动同步代码库中的 Values，默认值为 `false` | 否 |

`other` 类型代码源支持手动导入，不支持自动同步，服务端会将 `auto_sync` 设置为 `false`。

**Body 参数示例**

::: details

```json
{
  "codehost_name": "gitlab",
  "namespace": "demo",
  "repo": "backend-config",
  "branch": "main",
  "value_path": "helm/dev-values.yaml",
  "auto_sync": true
}
```

:::

**返回示例**

```json
{
  "message": "success"
}
```

#### 生产环境

**请求**

```
PUT /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

路径参数、Query 参数、Body 参数和返回参数与测试环境接口一致。

### 清除 Values 来源

清除 Git Values 来源和自动同步配置，保留当前环境 Values，不触发 Helm Release 更新。

#### 测试环境

**请求**

```
DELETE /openapi/environments/helm/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

路径参数和 Query 参数与获取测试环境 Values 来源接口一致。

**返回示例**

```json
{
  "message": "success"
}
```

#### 生产环境

**请求**

```
DELETE /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values-source?projectKey=<项目标识>
```

路径参数、Query 参数和返回参数与测试环境接口一致。

Values 来源接口不会返回代码源 ID、访问凭据、提交信息、Values 源文件内容或自动同步产生的内部状态。

## 预览环境服务 Values 更新

根据 Zadig 当前配置和更新参数，计算更新前后的完整 Values。该接口只生成预览，不更新服务配置、环境配置或 Helm Release。

::: warning
请求中的 `expected_revision` 必须与当前 Values 配置版本一致。请求至少需要设置 `values_yaml`、`override_kvs` 中的一项，或将 `sync_values_from_source`、`update_service_revision` 中的一项设置为 `true`。未传入 `values_yaml` 或 `override_kvs` 时保留当前配置；传入空字符串或空数组时清空对应配置。`values_yaml` 和 `sync_values_from_source` 不能同时设置。使用 `sync_values_from_source` 前需要先配置 Git Values 来源，且仅支持 `override` 合并策略。已开启自动同步时不能传入 `values_yaml`，需要先关闭自动同步或清除 Values 来源。该接口不会修改 Values 来源配置。在敏感字段中传入 `********` 表示保留该字段的当前值；当前配置不存在对应字段时，不能使用 `********`，必须传入真实值。
:::

### 测试环境

**请求**

```
POST /openapi/environments/helm/<环境标识>/services/<服务名称>/values/preview?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `expected_revision` | int | 期望的 Values 配置版本，必须与当前版本一致 | 是 |
| `values_yaml` | string | 目标环境覆盖 Values YAML，不包含 Chart 默认 Values 和服务配置 Values；不能与 `sync_values_from_source` 同时设置，未传时保留当前配置 | 否 |
| `override_kvs` | []KeyValue | 目标键值覆盖配置；优先级高于 `values_yaml`，未传时保留当前配置，传入空数组时清空 | 否 |
| `sync_values_from_source` | bool | 是否从已配置的 Git Values 来源读取目标 Values，默认值为 `false` | 否 |
| `update_service_revision` | bool | 是否使用服务配置的最新版本，默认值为 `false` | 否 |
| `value_merge_strategy` | string | `values_yaml` 的合并策略：`override` 替换当前环境 Values，`reuse-values` 将目标 Values 合并到当前环境 Values，默认值为 `override`；从 Git 来源同步时仅支持 `override` | 否 |

**KeyValue 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `key` | string | Values 键，使用点号表示嵌套路径 | 是 |
| `value` | string、number 或 bool | Values 标量值，不支持对象、数组或 `null` | 是 |

同一个 `override_kvs` 中的 `key` 不能重复。

**请求 Body 参数示例**

使用 `values_yaml` 预览：

::: details

```json
{
  "expected_revision": 3,
  "values_yaml": "replicaCount: 3\nimage:\n  tag: v1.3.0\ndatabase:\n  password: new-password\n",
  "update_service_revision": true,
  "value_merge_strategy": "override"
}
```

:::

使用 `override_kvs` 预览：

::: details

```json
{
  "expected_revision": 3,
  "override_kvs": [
    {
      "key": "resources.limits.cpu",
      "value": "1000m"
    }
  ]
}
```

:::

从已配置的 Git Values 来源读取时，请求示例如下：

::: details

```json
{
  "expected_revision": 3,
  "sync_values_from_source": true,
  "value_merge_strategy": "override"
}
```

:::

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `current_release_name` | string | 当前 Helm Release 名称 |
| `latest_release_name` | string | 更新后的 Helm Release 名称 |
| `current_values_yaml` | string | 根据 Zadig 当前配置计算的完整 Values YAML，敏感字段返回 `********` |
| `latest_values_yaml` | string | 更新后的完整 Values YAML，敏感字段返回 `********` |

**返回示例**

::: details

```json
{
  "current_release_name": "demo-backend-dev",
  "latest_release_name": "demo-backend-dev",
  "current_values_yaml": "replicaCount: 2\nimage:\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "latest_values_yaml": "replicaCount: 3\nimage:\n  tag: v1.3.0\ndatabase:\n  password: '********'\nresources:\n  limits:\n    cpu: 1000m\n"
}
```

:::

### 生产环境

**请求**

```
POST /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values/preview?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `expected_revision` | int | 期望的 Values 配置版本，必须与当前版本一致 | 是 |
| `values_yaml` | string | 目标环境覆盖 Values YAML，不包含 Chart 默认 Values 和服务配置 Values；不能与 `sync_values_from_source` 同时设置，未传时保留当前配置 | 否 |
| `override_kvs` | []KeyValue | 目标键值覆盖配置；优先级高于 `values_yaml`，未传时保留当前配置，传入空数组时清空 | 否 |
| `sync_values_from_source` | bool | 是否从已配置的 Git Values 来源读取目标 Values，默认值为 `false` | 否 |
| `update_service_revision` | bool | 是否使用服务配置的最新版本，默认值为 `false` | 否 |
| `value_merge_strategy` | string | `values_yaml` 的合并策略：`override` 替换当前环境 Values，`reuse-values` 将目标 Values 合并到当前环境 Values，默认值为 `override`；从 Git 来源同步时仅支持 `override` | 否 |

**返回说明**

| 参数名 | 类型 | 描述 |
| ------ | ---- | ---- |
| `current_release_name` | string | 当前 Helm Release 名称 |
| `latest_release_name` | string | 更新后的 Helm Release 名称 |
| `current_values_yaml` | string | 根据 Zadig 当前配置计算的完整 Values YAML，敏感字段返回 `********` |
| `latest_values_yaml` | string | 更新后的完整 Values YAML，敏感字段返回 `********` |

**返回示例**

::: details

```json
{
  "current_release_name": "demo-backend-prod",
  "latest_release_name": "demo-backend-prod",
  "current_values_yaml": "replicaCount: 3\nimage:\n  tag: v1.2.0\ndatabase:\n  password: '********'\n",
  "latest_values_yaml": "replicaCount: 4\nimage:\n  tag: v1.3.0\ndatabase:\n  password: '********'\n"
}
```

:::

## 更新环境服务 Values

保存环境服务 Values 配置并触发 Helm Release 更新。接口成功返回表示更新已受理，环境状态为 `success` 表示更新成功，`failed` 表示更新失败。

::: warning
请求中的 `expected_revision` 必须与当前 Values 配置版本一致。请求至少需要设置 `values_yaml`、`override_kvs` 中的一项，或将 `sync_values_from_source`、`update_service_revision` 中的一项设置为 `true`。未传入 `values_yaml` 或 `override_kvs` 时保留当前配置；传入空字符串或空数组时清空对应配置。`values_yaml` 和 `sync_values_from_source` 不能同时设置。使用 `sync_values_from_source` 前需要先配置 Git Values 来源，且仅支持 `override` 合并策略。已开启自动同步时不能传入 `values_yaml`，需要先关闭自动同步或清除 Values 来源。该接口不会修改 Values 来源配置。在敏感字段中传入 `********` 表示保留该字段的当前值；当前配置不存在对应字段时，不能使用 `********`，必须传入真实值。
:::

**Body 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `expected_revision` | int | 期望的 Values 配置版本，必须与当前版本一致 | 是 |
| `values_yaml` | string | 目标环境覆盖 Values YAML，不包含 Chart 默认 Values 和服务配置 Values；不能与 `sync_values_from_source` 同时设置，未传时保留当前配置 | 否 |
| `override_kvs` | []KeyValue | 目标键值覆盖配置；优先级高于 `values_yaml`，未传时保留当前配置，传入空数组时清空 | 否 |
| `sync_values_from_source` | bool | 是否从已配置的 Git Values 来源读取目标 Values，默认值为 `false` | 否 |
| `update_service_revision` | bool | 是否使用服务配置的最新版本，默认值为 `false` | 否 |
| `value_merge_strategy` | string | `values_yaml` 的合并策略：`override` 替换当前环境 Values，`reuse-values` 将目标 Values 合并到当前环境 Values，默认值为 `override`；从 Git 来源同步时仅支持 `override` | 否 |

**KeyValue 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `key` | string | Values 键，使用点号表示嵌套路径 | 是 |
| `value` | string、number 或 bool | Values 标量值，不支持对象、数组或 `null` | 是 |

同一个 `override_kvs` 中的 `key` 不能重复。

### 测试环境

**请求**

```
PUT /openapi/environments/helm/<环境标识>/services/<服务名称>/values?projectKey=<项目标识>
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `envName` | string | 环境标识 | 是 |
| `serviceName` | string | Zadig Helm 服务名称 | 是 |

**Query 参数说明**

| 参数名 | 类型 | 描述 | 必填 |
| ------ | ---- | ---- | ---- |
| `projectKey` | string | 项目标识 | 是 |

**Body 参数示例**

::: details

```json
{
  "expected_revision": 3,
  "sync_values_from_source": true,
  "override_kvs": [
    {
      "key": "resources.limits.cpu",
      "value": "1000m"
    }
  ],
  "update_service_revision": true,
  "value_merge_strategy": "override"
}
```

:::

**返回示例**

```json
{
  "message": "success"
}
```

### 生产环境

**请求**

```
PUT /openapi/environments/helm/production/<环境标识>/services/<服务名称>/values?projectKey=<项目标识>
```

路径参数、Query 参数和 Body 参数与测试环境接口一致。

**返回示例**

```json
{
  "message": "success"
}
```

## 错误响应

| HTTP 状态码 | 说明 |
| --- | --- |
| `400` | 参数错误、YAML 内容无效、目标资源不存在、生产功能不可用或其他业务校验失败 |
| `403` | 没有对应资源权限 |
| `404` | 请求路径不存在 |
| `409` | `expected_revision` 与当前服务版本或 Values 配置版本不一致，或提交 Values 更新时环境正在更新 |
| `500` | 服务端内部错误 |
