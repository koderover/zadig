# 工作流任务使用阿里云 RRSA

阿里云 ACK 集群复用现有 IRSA 配置字段，为 freestyle 类工作流任务提供 RAM 角色身份，无需在任务中保存长期 AccessKey。插件任务不接入 RRSA，保持原有 ServiceAccount 和权限，与现有 AWS IRSA 的支持范围一致。

## 前置配置

1. 在任务执行的 ACK 集群开启 RRSA，安装 `ack-pod-identity-webhook` 0.2.0 或以上版本。
2. 在与 ACK 集群相同的阿里云账号下创建 RAM 角色，授予任务所需的云资源权限。本实现通过角色名注解关联角色，仅支持同账号角色，不支持通过填写跨账号 ARN 选择其他账号的角色。
3. 配置角色的 OIDC 信任策略，限定集群的身份提供商及 ServiceAccount：
   - 外部集群：`system:serviceaccount:koderover-agent:workflow-cm-sa`。
   - 本地集群：将 `koderover-agent` 替换为 Zadig 的安装命名空间。
4. 确认任务容器可以访问 STS，使用支持 RRSA 的阿里云 SDK 或 CLI。

具体操作参考[阿里云 RRSA 文档](https://help.aliyun.com/zh/ack/ack-managed-and-ack-dedicated/user-guide/use-rrsa-to-authorize-pods-to-access-different-cloud-services)。

## Zadig 配置

通过集群创建或更新 API 配置阿里云 ACK 的 RRSA，保存后将工作流任务调度到该集群。本次仅提供后端支持，不新增前端配置入口。

API 继续使用现有字段，将以下字段合入完整的集群请求，并保留其他配置：

```json
{
  "provider": 1,
  "advanced_config": {
    "schedule_workflow": true,
    "enable_irsa": true,
    "irsa_role_arn": "acs:ram::1234567890123456:role/zadig-build"
  }
}
```

Zadig 为 `workflow-cm-sa` 配置 `pod-identity.alibabacloud.com/role-name` 注解，并为任务 Pod 添加 `pod-identity.alibabacloud.com/injection: 'on'` 标签。该配置不修改命名空间标签。Webhook 挂载 OIDC Token 并注入角色配置，由任务中的 SDK 或 CLI 换取和刷新临时凭证。

角色变更或关闭 RRSA 并成功更新集群资源后，新创建的任务使用新配置；已经运行的任务不会被重建。使用相同 ServiceAccount 的任务共享角色权限。

集群配置先保存到数据库，再通过 `UpgradeAgent` 更新 ServiceAccount。如果集群状态不是 Normal，或资源更新失败，数据库配置与实际 ServiceAccount 注解可能不一致，任务可能无法获得凭证，也可能继续使用旧角色。遇到保存或更新报错时，应待集群恢复后重新保存配置，并确认 ServiceAccount 注解更新成功，再执行任务。此处沿用现有 AWS IRSA 的更新机制。

此功能覆盖工作流任务的云身份接入，不改变 Zadig 内置对象存储的 AK/SK 认证方式。AWS IRSA 继续使用原有配置。
