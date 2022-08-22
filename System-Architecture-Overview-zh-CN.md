# Zadig 系统架构简介

## 系统架构图

![Architecture_diagram](./Zadig-System-Architecture.svg)

## 核心组件介绍

用户入口：
- zadig-portal：Zadig 前端组件
- Zadig Toolkit：vscode 开发者插件

API 网关：
- [Gloo Edge](https://github.com/solo-io/gloo)：Zadig 的 API 网关组件
- [OPA](https://github.com/open-policy-agent/opa)：认证和授权组件
- [Dex](https://github.com/dexidp/dex)：Zadig 的身份认证服务，用于连接其他第三方认证系统，比如 AD / LDAP / OAuth2 / GitHub / ..

Zadig 核心业务：
- Aslan：项目 / 环境 / 服务 / 工作流 / 构建配置 / 系统配置等系统功能
- Config：系统配置
- Workflow Runner：
  - warpdrive：工作流引擎，负责 reaper、predator 实例的创建销毁等管理操作
  - reaper：负责执行单个工作流作业中的构建、测试等任务
  - predator：负责执行单个工作流作业中的镜像分发任务
  - plugins：工作流插件
    - Jenkins-plugin：用于触发 Jenkins job，显示状态和结果等
- Cron：定时任务，包括环境的回收，K8s 资源的清理等
- NSQ：消息队列（第三方组件）

数据平面：
- MongoDB：业务数据数据库
- MySQL：存储 dex 配置、用户信息的数据库

K8s 集群：
- Zadig 业务运行在各种云厂商的标准 K8s 集群