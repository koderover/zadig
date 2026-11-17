
# Zadig 开发流程

- [1. 克隆代码](#1-克隆代码)
- [2. 本地开发环境搭建](#2-本地开发环境搭建)
- [3. 贡献代码](#3-贡献代码)
- [4. 镜像构建及部署](#4-镜像构建及部署)
    - [后端构建](#后端构建)
    - [镜像部署](#镜像部署)

## 1. 克隆代码

1. 在 [koderover/zadig](https://github.com/koderover/zadig) 仓库，点击右上角的 Fork 按钮。

```bash
git clone git@github.com:<your_github_id>/zadig.git
```

## 2. 本地开发环境搭建

Zadig 后端使用 Go 语言，在您贡献代码之前，本地需安装 Go 1.15+ 版本。

## 3. 贡献代码

请详细阅读 [代码贡献指南](../../CONTRIBUTING-zh-CN.md) 并遵循上面的流程。

## 4. 镜像构建及部署

### 后端构建
> 服务列表：aslan cron hub-server hub-agent predator-plugin ua
> 请确认当前构建环境有推送镜像至开发环境的远端仓库的权限

1. 执行 `export IMAGE_REPOSITORY={YOUR_IMAGE_REGISTRY_URL}`指定目标镜像仓库地址
2. 执行 `export VERSION={YOUR_IMAGE_TAG}` 指定镜像 TAG
3. 在 zadig 代码库根目录执行 `make {SERVICE}.push` 构建出镜像并上传至镜像仓库

构建后端镜像时，`make {SERVICE}.image`、`make {SERVICE}.push` 和 `make {SERVICE}.dev` 会自动写入以下信息：

| 环境变量 | 镜像标签 | 内容 |
|----------|----------|------|
| `BUILD_VERSION` | `org.opencontainers.image.version` | 与 `VERSION` 构建参数一致，默认使用时间戳 |
| `BUILD_COMMIT` | `org.opencontainers.image.revision` | 完整 commit ID |
| `BUILD_BRANCH` | `org.opencontainers.image.ref.name` | Git 分支名 |
| `BUILD_DATE` | `org.opencontainers.image.created` | UTC 构建时间（RFC 3339） |

CI 使用 detached HEAD 时，请显式传入分支名，例如：

```bash
make aslan.push VERSION=v4.0.0 GIT_BRANCH=release/v4.0.0
```

Makefile 对外保留 `VERSION`、`GIT_COMMIT`、`GIT_BRANCH`、`BUILD_DATE`，均支持通过环境变量或 Make 参数覆盖；传入 Docker 时分别映射为 `BUILD_VERSION`、`BUILD_COMMIT`、`BUILD_BRANCH`、`BUILD_DATE`。直接运行 Docker 构建时需传入对应的 `--build-arg`，未提供的信息为 `unknown`：

```bash
docker build -f docker/aslan.Dockerfile -t zadig-aslan:dev \
  --build-arg BUILD_VERSION=dev \
  --build-arg BUILD_COMMIT="$(git rev-parse HEAD)" \
  --build-arg BUILD_BRANCH="$(git symbolic-ref --quiet --short HEAD || echo unknown)" \
  --build-arg BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')" .
```

查看镜像标签或容器内的构建信息：

```bash
docker image inspect <image> --format '{{json .Config.Labels}}'
docker run --rm --entrypoint sh <image> -c "printenv | grep '^BUILD_'"
```

### 镜像部署

将构建出的服务镜像替换至相关 workload 资源
服务以及 workload 对应关系如下：

| 服务名          | workload 资源              |
|-----------------|----------------------------|
| aslan           | deployment/aslan           |
| cron            | deployment/cron            |
| hub-server      | deployment/hub-server      |




