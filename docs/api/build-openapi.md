# 构建 OpenAPI

本文描述构建和构建模板 OpenAPI 中镜像构建步骤的代理开关。

## 适用接口

`enable_proxy` 是 `docker_build_step` 对象中的可选字段，适用于以下请求和响应：

| 接口 | 方法 | 说明 |
| --- | --- | --- |
| `/openapi/build?projectKey=<项目标识>` | `POST` | 新建构建 |
| `/openapi/build?projectKey=<项目标识>` | `PUT` | 更新构建 |
| `/openapi/build/<构建名称>/detail?projectKey=<项目标识>` | `GET` | 获取构建详情 |
| `/openapi/templates/builds` | `POST` | 新建构建模板 |
| `/openapi/templates/builds/<构建模板 ID>` | `PUT` | 更新构建模板 |
| `/openapi/templates/builds/<构建模板 ID>` | `GET` | 获取构建模板详情 |

使用构建模板新建或更新构建时，代理开关随模板中的 `docker_build_step.enable_proxy` 生效。

## DockerBuildStep 参数说明

`docker_build_step` 用于配置镜像构建步骤。不构建镜像时无需传入该对象。

| 参数名 | 说明 | 类型 | 必填 |
| --- | --- | --- | --- |
| `dockerfile_source` | Dockerfile 来源，支持 `local`、`template` | string | 是 |
| `build_context_dir` | 构建上下文目录 | string | 是 |
| `dockerfile_directory` | Dockerfile 绝对路径，`dockerfile_source=local` 时必填 | string | 否（本地来源时必填） |
| `template_name` | Dockerfile 模板名称，`dockerfile_source=template` 时必填 | string | 否 |
| `build_args` | Docker 构建参数 | string | 否 |
| `enable_proxy` | 是否为当前镜像构建步骤注入代理 build-arg | bool | 否 |
| `enable_buildkit` | 是否启用 BuildKit | bool | 否 |
| `platforms` | BuildKit 构建平台，例如 `linux/amd64`；启用 BuildKit 时必填 | string | 否（启用 BuildKit 时必填） |

### `enable_proxy` 行为

- 未传入或传入 `null`：沿用历史行为，由系统代理配置中的 `enable_repo_proxy` 决定是否注入代理。
- 传入 `true`：仅当系统代理配置已开启 `enable_repo_proxy` 时，为当前镜像构建步骤注入代理。
- 传入 `false`：关闭当前镜像构建步骤的代理注入，不影响其他构建步骤或代码源代理。
- 代理类型为 `http` 或 `https` 时，注入 `http_proxy` 和 `https_proxy` 两个 Docker build-arg。
- 代理类型为 `socks5` 时，不会将 SOCKS5 地址作为 Docker build-arg 传入。

该字段只控制 Docker build 的代理 build-arg，不控制 Git 拉取、软件包安装、Docker 登录/推送或 DinD 镜像拉取。

## 新建构建

**请求**

```text
POST /openapi/build?projectKey=<项目标识>
```

请求 body 中的 `docker_build_step` 使用上述参数。示例：

```json
{
  "name": "demo-build",
  "project_key": "demo",
  "infrastructure": "kubernetes",
  "build_os": "ubuntu 20.04",
  "script_type": "shell",
  "build_script": "set -e",
  "services": [],
  "docker_build_step": {
    "dockerfile_source": "local",
    "build_context_dir": "$REPONAME_0",
    "dockerfile_directory": "$REPONAME_0/Dockerfile",
    "build_args": "",
    "enable_proxy": false,
    "enable_buildkit": false,
    "platforms": ""
  }
}
```

## 更新构建

**请求**

```text
PUT /openapi/build?projectKey=<项目标识>
```

更新请求按完整构建配置处理。需要保留历史代理行为时可以省略 `enable_proxy`；需要关闭当前镜像构建步骤的代理时传入 `false`。

## 获取构建详情

**请求**

```text
GET /openapi/build/<构建名称>/detail?projectKey=<项目标识>
```

如果构建包含镜像构建步骤，详情响应中的 `post_build.docker_build` 返回 `enable_proxy`：

```json
{
  "post_build": {
    "docker_build": {
      "work_dir": "$REPONAME_0",
      "docker_file": "$REPONAME_0/Dockerfile",
      "build_args": "",
      "enable_proxy": false,
      "source": "local",
      "template_name": ""
    }
  }
}
```

## 构建模板接口

构建模板接口使用相同的 `DockerBuildStep` 参数结构：

```text
GET  /openapi/templates/builds/<构建模板 ID>
POST /openapi/templates/builds
PUT  /openapi/templates/builds/<构建模板 ID>
```

在模板请求 body 中，将 `enable_proxy` 放在 `docker_build_step` 下。例如：

```json
{
  "name": "demo-template",
  "infrastructure": "kubernetes",
  "build_os": "ubuntu 20.04",
  "script_type": "shell",
  "build_script": "set -e",
  "docker_build_step": {
    "dockerfile_source": "local",
    "build_context_dir": "$WORKSPACE",
    "dockerfile_directory": "$WORKSPACE/Dockerfile",
    "enable_proxy": true,
    "enable_buildkit": false,
    "platforms": ""
  }
}
```

模板详情响应会原样返回已保存的 `enable_proxy` 值。旧模板没有该字段时，响应可以不包含该字段，使用模板执行构建时继续沿用历史行为。

## 权限和错误响应

构建和构建模板接口沿用现有 OpenAPI 的身份认证、资源权限和错误响应约定。本次只新增可选字段，没有新增接口、权限要求或错误码。
