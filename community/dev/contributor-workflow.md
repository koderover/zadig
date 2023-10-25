
# Zadig 开发流程

- [1. 克隆代码](#1-克隆代码)
- [2. 本地开发环境搭建](#2-本地开发环境搭建)
  - [前端环境](#前端环境)
  - [后端环境](#后端环境)
- [3. 贡献代码](#3-贡献代码)
- [4. 镜像构建及部署](#4-镜像构建及部署)
    - [前端构建](#前端构建)
    - [后端构建](#后端构建)
    - [镜像部署](#镜像部署)

## 1. 克隆代码

1. 在 [koderover/zadig](https://github.com/koderover/zadig) 仓库，点击右上角的 Fork 按钮。
2. 在 [koderover/zadig-portal](https://github.com/koderover/zadig-portal) 仓库，点击右上角的 Fork 按钮。
2. 分别将 fork 后的前后端代码仓库克隆到本地

```bash
git clone git@github.com:<your_github_id>/zadig.git
git clone git@github.com:<your_github_id>/zadig-portal.git
```

## 2. 本地开发环境搭建

### 前端环境

Zadig 前端使用的 Vue 框架，在您贡献代码之前，本地需安装 Node.js 14+、Yarn。

注意：我们使用 Yarn 进行依赖版本的锁定，所以请使用 Yarn 安装依赖。

### 后端环境

Zadig 后端使用 Go 语言，在您贡献代码之前，本地需安装 Go 1.15+ 版本。

## 3. 贡献代码

请详细阅读 [代码贡献指南](../../CONTRIBUTING-zh-CN.md) 并遵循上面的流程。

## 4. 镜像构建及部署

### 后端构建
> 服务列表：aslan cron hub-server hub-agent resource-server predator-plugin ua warpdrive
> 请确认当前构建环境有推送镜像至开发环境的远端仓库的权限

1. 执行 `export IMAGE_REPOSITORY={YOUR_IMAGE_REGISTRY_URL}`指定目标镜像仓库地址
2. 执行 `export VERSION={YOUR_IMAGE_TAG}` 指定镜像 TAG
3. 在 zadig 代码库根目录执行 `make {SERVICE}.push` 构建出镜像并上传至镜像仓库

### 前端构建
> 请确认当前构建环境有推送镜像至远端仓库的权限
1. 切换至 zadig-portal 目录
2. 执行 `export IMAGE_REPOSITORY={YOUR_IMAGE_REGISTRY_URL}`指定目标镜像仓库地址
3. 执行 `export VERSION={YOUR_IMAGE_TAG}` 指定镜像 TAG
4. 执行 make all 构建镜像并上传至镜像仓库

### 镜像部署

将构建出的服务镜像替换至相关 workload 资源
服务以及 workload 对应关系如下：

| 服务名          | workload 资源              |
|-----------------|----------------------------|
| aslan           | deployment/aslan           |
| cron            | deployment/corn            |
| hub-server      | deployment/hub-server      |
| resource-server | deployment/resource-server |
| warpdrive       | deployment/warpdrive       |
| zadig-portal    | deployment/zadig-portal    |




