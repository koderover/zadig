# Zadig 系统架构简介

## 系统架构图

![Architecture_diagram](./Zadig-System-Architecture.png)

## 核心组件介绍
用户入口：
- zadig-portal：Zadig 前端组件
- Zadig Toolkit：vscode 开发者插件

API 网关：
- [Gloo Edge](https://github.com/solo-io/gloo)：Zadig 的 API 网关组件
- Zadig User Service：用户认证和授权组件
- [Dex](https://github.com/dexidp/dex)：Zadig 的身份认证服务，用于连接其他第三方认证系统，比如 AD，LDAP，OAuth2，GitHub，...

Zadig 核心业务：
- Aslan：项目，环境，服务，工作流，构建配置，系统配置等系统功能
- Workflow Runner：
  - job-executor：用于在 K8s Pod 上执行自定义工作流任务的组件
  - Job-agent: 用于在虚拟机上执行自定义工作流任务的组件
- Cron：定时任务，包括环境的回收，K8s 资源的清理等
- NSQ：消息队列（第三方组件）

数据平面：
- MongoDB：业务数据数据库
- MySQL：存储 dex 配置、用户信息的数据库

K8s 集群：
- Zadig 业务运行在各种云厂商的标准 K8s 集群
