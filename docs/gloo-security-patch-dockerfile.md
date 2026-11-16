# Gloo 系统包补丁 Dockerfile 使用说明

本文介绍如何使用 `docker/gloo-security-patch.Dockerfile`，基于原有 Gloo 镜像升级指定的 Alpine 系统包。

适用于本次补丁使用的 `gateway`、`discovery`、`gloo` 和 `gloo-envoy-wrapper` 镜像。

## 准备工作

- 安装并启动 Docker，确保可以使用 `docker buildx`。
- 确认当前代码目录包含 `docker/gloo-security-patch.Dockerfile`。
- 准备原始镜像地址及 digest，以及需要升级的软件包和固定版本。

> 该 Dockerfile 只更新系统包，不升级 Gloo、Envoy 程序版本。构建需要能够访问原镜像仓库和镜像中配置的 APK 软件源。

## 构建参数

| 参数 | 说明 | 示例 |
| --- | --- | --- |
| `BASE_IMAGE` | 原始镜像地址，使用 digest 固定构建来源 | `镜像仓库/组件@sha256:...` |
| `APK_PACKAGES` | 需要升级的软件包，按 `包名=版本号` 填写，多个包使用空格分隔 | `busybox=1.32.1-r9 zlib=1.2.12-r3` |

构建时临时使用 root 用户执行 `apk add --upgrade --no-cache`，完成后切换为 `10101` 用户，并继承基础镜像的 `ENTRYPOINT` 和 `CMD`。

`APK_PACKAGES` 未填写或为空时，构建会失败。Dockerfile 不会自动校验每个包是否指定了固定版本，需要在传参时保证格式正确。

## 构建镜像

以下以 `gateway` 的 amd64 补丁镜像为例，在包含 Dockerfile 的代码仓库根目录执行。

**第一步**：设置基础镜像和软件包参数。

```bash
BASE_IMAGE='koderover.tencentcloudcr.com/koderover-public/gateway@sha256:96eba53042b0108548aeb049440091addd3b838b61f17ca815a68aef204f0b7e'

APK_PACKAGES='busybox=1.32.1-r9 ssl_client=1.32.1-r9 libcrypto1.1=1.1.1s-r0 libssl1.1=1.1.1s-r0 zlib=1.2.12-r3'
```

上述软件包版本为本次补丁实际使用的版本。后续构建新补丁时，需要根据待修复漏洞和原镜像的 Alpine 分支重新确认，不能将其作为长期通用的修复版本。

**第二步**：执行构建。

```bash
docker buildx build \
  --platform linux/amd64 \
  --build-arg "BASE_IMAGE=${BASE_IMAGE}" \
  --build-arg "APK_PACKAGES=${APK_PACKAGES}" \
  --file docker/gloo-security-patch.Dockerfile \
  --tag gloo-security-patch/gateway:local-amd64 \
  --load \
  .
```

参数说明：

- `--platform`：本次构建的目标架构。
- `--build-arg`：传入基础镜像和软件包参数。
- `--file`：指定 Dockerfile 路径。
- `--tag`：设置构建结果的镜像名称和标签，使用与原镜像不同的标签。
- `--load`：将构建结果加载到本地 Docker。
- `.`：使用当前目录作为构建上下文。

构建成功后，本地镜像为 `gloo-security-patch/gateway:local-amd64`。此命令只在本地构建，不会推送镜像或更新环境。

## 构建其他组件和架构

四个组件共用同一个 Dockerfile。构建其他组件时，替换 `BASE_IMAGE`、`APK_PACKAGES` 和 `--tag`；构建 arm64 时，同时将 `--platform` 改为 `linux/arm64`，并使用能够执行该架构构建的节点。

原始镜像的 Alpine 分支存在以下差异：

| 架构 | 组件 | Alpine 分支 |
| --- | --- | --- |
| amd64 | gateway、discovery、gloo、gloo-envoy-wrapper | 3.13 |
| arm64 | gateway、discovery | 3.13 |
| arm64 | gloo | 3.12 |
| arm64 | gloo-envoy-wrapper | 3.14 |

基础镜像必须包含目标架构，软件包版本必须匹配对应 Alpine 分支。不同组件和架构的参数不能直接通用；包版本不可获取时，应核对版本和原软件源，不要直接更换 Alpine 分支。

> 当前 Dockerfile 最终固定使用 `10101` 用户，适用于上述已核对运行用户的 Gloo 镜像，不应直接套用于运行用户不同的其他镜像。
