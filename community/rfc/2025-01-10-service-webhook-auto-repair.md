# Service Webhook 自动修复机制

- Author: 夏佳怡
- Issue: 服务模板 webhook 被手动删除后无法自动恢复
- Date: 2025-10-10
- Reviewer:
- Review status: pending

## Objective

当服务模板(Service Template)的 webhook 在代码仓库(GitLab/GitHub/Gerrit/Gitee)中被手动删除后,系统能够在下次更新服务时自动检测并重建 webhook,确保 webhook 功能的持续可用性。

## Background

### 当前问题

Zadig 在添加服务时会在对应的代码仓库创建 webhook,用于监听代码变更并触发相应的工作流。但存在以下问题:

1. **问题根源**: 当 webhook 在代码仓库中被手动删除后(如清理、误删、仓库迁移等场景),系统无法自动恢复
2. **现有机制缺陷**: 更新服务时,系统只对比数据库中的 webhook 记录与期望的记录,而不检查代码仓库中 webhook 的实际状态
3. **影响**: webhook 失效后,代码变更无法触发工作流,用户需要手动删除服务并重新创建才能恢复

### 当前实现

服务更新调用链:
```
PUT /api/aslan/service/loader/load/:codehostId
  → SyncServiceTemplate (handler)
    → LoadServiceFromCodeHost (service layer)
      → loadService / loadGerritService / loadGiteeService
        → CreateServiceTemplate
          → ProcessServiceWebhook
            → ProcessWebhook (只对比数据库记录,不验证仓库实际状态)
```

### 重要发现

系统已经在 `webhook` MongoDB 集合中存储了完整的 webhook 信息:

```go
type WebHook struct {
    ID         primitive.ObjectID `bson:"_id,omitempty"`
    Owner      string             `bson:"owner"`
    Repo       string             `bson:"repo"`
    Address    string             `bson:"address"`
    HookID     string             `bson:"hook_id,omitempty"`  // Webhook 在 Git 平台的实际 ID
    References []string           `bson:"references"`          // 引用此 webhook 的服务列表
}
```

这意味着无需修改 Service 模型,可以直接利用现有的 `HookID` 字段进行验证。

## Design overview

### 核心思路

在更新服务时,增加 webhook 存在性验证环节:
1. 从数据库查询 webhook 记录及其 HookID
2. 调用 Git 平台 API 验证 webhook 是否真实存在
3. 如果不存在,自动重建 webhook

### 关键约束

1. **非侵入性**: 验证失败不应影响服务更新主流程
2. **执行时机**: 仅在**更新**服务时验证(不在创建时验证,因为创建时 webhook 是新建的)
3. **异步执行**: 使用 goroutine 异步执行验证,不阻塞服务更新响应
4. **多平台支持**: 支持 GitHub、GitLab、Gerrit、Gitee 公有云、Gitee 私有部署

### 成功标准

1. 用户更新服务时,如果 webhook 已被删除,系统能自动重建
2. 重建过程对用户透明,无需手动干预
3. 验证/重建失败不影响服务更新流程
4. 所有代码托管平台(GitHub、GitLab、Gerrit、Gitee 公有云、Gitee 私有部署)均正常工作

## Detailed design

### 整体流程

```
更新服务操作
  ↓
ProcessServiceWebhook (现有逻辑)
  ├─ ProcessWebhook (现有: 对比数据库记录,增删 webhook)
  └─ [新增] 验证并修复 webhook
       ├─ 查询数据库中的 webhook 记录(获取 HookID)
       ├─ 调用 Git 平台 API 验证 webhook 是否存在
       └─ 如果不存在: 删除旧记录 → 重新创建 webhook
```

### 实现步骤

#### 1. 为各 Git 平台客户端添加验证接口

**GitLab** (`pkg/microservice/aslan/core/common/service/gitlab/webhook.go`):
```go
func (c *Client) WebHookExists(owner, repo, hookID string) (bool, error) {
    if hookID == "" {
        return false, nil
    }

    hookIDInt, err := strconv.Atoi(hookID)
    if err != nil {
        return false, err
    }

    _, err = c.GetProjectHook(owner, repo, hookIDInt)
    if err != nil {
        // 404 错误说明 webhook 不存在
        return false, nil
    }
    return true, nil
}
```

#### 2. 修改 ProcessServiceWebhook

**位置**: `pkg/microservice/aslan/core/common/service/service.go:752`

```go
func ProcessServiceWebhook(updated, current *commonmodels.Service, serviceName string, production bool, logger *zap.SugaredLogger) {
    // ... 现有逻辑保持不变 ...

    err := ProcessWebhook(updatedHooks, currentHooks, name, logger)
    if err != nil {
        logger.Errorf("Failed to process WebHook, error: %s", err)
    }

    // 新增: 仅在更新服务时验证并修复 webhook
    // 异步执行,不阻塞主流程,失败也不影响服务更新
    if updated != nil && current != nil {
        go func(svc *commonmodels.Service, name string, prod bool, log *zap.SugaredLogger) {
            if err := VerifyAndRepairWebhook(svc, name, prod, log); err != nil {
                log.Warnf("Failed to verify and repair webhook for service %s, error: %s", name, err)
            }
        }(updated, serviceName, production, logger)
    }
}
```

**关键点**:
- 条件 `updated != nil && current != nil` 确保只在更新服务时执行
- **异步执行 goroutine**: 不阻塞服务更新主流程,提升响应速度
- 验证失败只记录 Warning,不影响主流程
- 在 ProcessWebhook 之后调用,确保主流程的增删操作已完成
- 通过参数传递避免闭包捕获变量的并发问题

#### 3. 实现验证修复函数

**位置**: `pkg/microservice/aslan/core/common/service/service.go`

```go
func VerifyAndRepairWebhook(service *commonmodels.Service, serviceName string, production bool, logger *zap.SugaredLogger) error {
    if !needProcessWebhook(service.Source) {
        return nil
    }

    // 1. 根据 Service 查询 webhook 记录
    namespace := service.GetRepoNamespace()
    repoName := service.RepoName
    address, err := GetGitlabAddress(service.SrcPath)
    if err != nil || address == "" {
        return fmt.Errorf("failed to get gitlab address: %w", err)
    }

    // 2. 查询数据库中的 webhook (包含 HookID)
    webhookRecord, err := mongodb.NewWebHookColl().Find(namespace, repoName, address)
    if err != nil {
        logger.Warnf("Failed to find webhook record in database: %v", err)
        return nil // 数据库没有记录,说明未创建过,无需修复
    }

    if webhookRecord.HookID == "" {
        logger.Warnf("Webhook record exists but has no HookID, skipping verification")
        return nil
    }

    // 3. 获取 codehost 配置
    ch, err := systemconfig.New().GetCodeHost(service.CodehostID)
    if err != nil {
        return fmt.Errorf("failed to get codehost: %w", err)
    }

    // 4. 验证 webhook 是否存在
    exists, err := checkWebhookExists(ch, service, webhookRecord.HookID, logger)
    if err != nil {
        logger.Warnf("Failed to check webhook existence: %v", err)
        return err
    }

    // 5. 如果不存在,重新创建
    if !exists {
        logger.Infof("Webhook not found in repository (HookID: %s), recreating for service %s",
            webhookRecord.HookID, serviceName)

        name := webhook.ServicePrefix + serviceName
        if production {
            name = webhook.ServicePrefix + "production-" + serviceName
        }

        // 删除旧记录
        err = mongodb.NewWebHookColl().Delete(namespace, repoName, address)
        if err != nil {
            logger.Warnf("Failed to delete old webhook record: %v", err)
        }

        // 重新创建
        err = webhook.NewClient().AddWebHook(&webhook.TaskOption{
            ID:        ch.ID,
            Owner:     service.RepoOwner,
            Namespace: namespace,
            Repo:      repoName,
            Address:   ch.Address,
            Token:     ch.AccessToken,
            Ref:       name,
            AK:        ch.AccessKey,
            SK:        ch.SecretKey,
            Region:    ch.Region,
            From:      ch.Type,
        })
        if err != nil {
            return fmt.Errorf("failed to recreate webhook: %w", err)
        }
        logger.Infof("Successfully recreated webhook for service %s", serviceName)
    }

    return nil
}

func checkWebhookExists(ch *systemconfig.CodeHost, service *commonmodels.Service, hookID string, logger *zap.SugaredLogger) (bool, error) {
    namespace := service.GetRepoNamespace()
    repo := service.RepoName

    switch ch.Type {
    case setting.SourceFromGitlab:
        client, err := gitlabservice.NewClient(ch.ID, ch.Address, ch.AccessToken,
            config.ProxyHTTPSAddr(), ch.EnableProxy, ch.DisableSSL)
        if err != nil {
            return false, fmt.Errorf("failed to create gitlab client: %w", err)
        }
        return client.WebHookExists(namespace, repo, hookID)

    case setting.SourceFromGithub:
        client := githubservice.NewClient(ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
        return client.WebHookExists(namespace, repo, hookID)

    case setting.SourceFromGerrit:
        client := gerrit.NewClient(ch.Address, ch.AccessToken, config.ProxyHTTPSAddr(), ch.EnableProxy)
        remoteName := service.GerritRemoteName
        if remoteName == "" {
            remoteName = "origin"
        }
        return client.WebHookExists(repo, remoteName, hookID)

    case setting.SourceFromGitee, setting.SourceFromGiteeEE:
        client := giteeservice.NewClient(ch.ID, ch.Address, ch.AccessToken,
            config.ProxyHTTPSAddr(), ch.EnableProxy)
        return client.WebHookExists(namespace, repo, hookID)

    default:
        logger.Warnf("Unsupported code source type: %s", ch.Type)
        return true, nil // 不支持的类型默认认为存在
    }
}
```

### API changes

无需修改 API。此功能在服务更新的现有流程中自动执行。

### Database changes

无需修改数据库。利用现有的 `webhook` 集合和 `HookID` 字段。

### Backward compatibility consideration

#### 完全向后兼容

1. **不影响现有流程**:
   - 验证逻辑作为可选增强,失败不影响服务更新
   - 所有现有功能保持不变

2. **数据兼容性**:
   - 使用现有的 webhook 表结构,无需迁移
   - 如果 HookID 为空(老数据),自动跳过验证

3. **行为兼容性**:
   - 只在更新服务时执行验证(不在创建时)
   - 不改变任何现有的 webhook 创建/删除逻辑

4. **回滚方案**:
   - 如果出现问题,可以通过代码开关禁用验证逻辑
   - 原有的 ProcessWebhook 逻辑完全保留

### Performance consideration

1. **API 调用开销**:
   - 每次更新服务增加 1 次 Git 平台 API 调用(验证 webhook)
   - **异步执行**: 不阻塞服务更新响应,用户无感知
   - API 验证通常响应时间 < 100ms

2. **失败场景**:
   - 需要重建时增加 2 次 API 调用(删除 + 创建)
   - 这是低频操作,只在 webhook 被删除时发生
   - 异步执行不影响用户体验

3. **并发考虑**:
   - 使用 goroutine 异步执行,避免阻塞主线程
   - 通过参数传递避免闭包变量竞争
   - 每个服务的验证操作是独立的,互不干扰

### Security and privacy consideration

1. **权限控制**:
   - 使用系统已配置的 codehost token
   - 不引入新的权限要求

2. **错误处理**:
   - API 调用失败不暴露敏感信息
   - 只记录必要的日志信息

3. **数据安全**:
   - 不修改任何敏感数据
   - webhook token 的处理与现有逻辑一致

## Implementation plan

### Phase 1: 核心功能实现(预计 2-3 天)

1. 为 GitLab/GitHub/Gerrit/Gitee 客户端添加 `WebHookExists` 方法
2. 实现 `VerifyAndRepairWebhook` 和 `checkWebhookExists` 函数
3. 修改 `ProcessServiceWebhook` 调用验证逻辑

### Phase 2: 测试验证(预计 2 天)

1. **单元测试**:
   - 测试各平台的 WebHookExists 方法
   - 测试验证修复逻辑的各种场景

2. **集成测试**:
   - 在测试环境创建服务
   - 手动删除 webhook
   - 更新服务验证自动重建

3. **多平台测试**:
   - GitHub: GitHub.com
   - GitLab: 自托管 + GitLab.com
   - Gerrit: 自托管 Gerrit 服务器
   - Gitee 公有云: Gitee.com
   - Gitee 私有部署: 企业版 Gitee

### Phase 3: 文档和发布(预计 1 天)

1. 更新用户文档,说明自动修复功能
2. 准备 changelog
3. 发布 PR

## Testing plan

### 单元测试

```go
func TestWebHookExists(t *testing.T) {
    // 测试 webhook 存在的情况
    // 测试 webhook 不存在的情况
    // 测试 hookID 为空的情况
    // 测试 API 调用失败的情况
}

func TestVerifyAndRepairWebhook(t *testing.T) {
    // 测试数据库无记录的情况
    // 测试 HookID 为空的情况
    // 测试 webhook 存在,无需修复
    // 测试 webhook 不存在,需要重建
}
```

### 集成测试场景

| 场景 | 操作步骤 | 预期结果 |
|------|---------|---------|
| 创建新服务 | 创建服务 | webhook 正常创建,不触发验证 |
| 更新服务(webhook 正常) | 更新服务 | 验证通过,无额外操作 |
| 更新服务(webhook 被删) | 1. 创建服务<br>2. 手动删除仓库中的 webhook<br>3. 更新服务 | 自动检测并重建 webhook |
| Webhook 记录异常 | HookID 为空或记录不存在 | 跳过验证,不影响更新 |

### 性能测试

- 测试 100 个服务并发更新的响应时间
- 确保 API 调用不会成为性能瓶颈

## Open questions

1. **API 限流**: 各 Git 平台都有 API rate limit,大规模使用时是否需要考虑?
   - **答**: 当前方案只在更新服务时验证,属于低频操作,暂不需要额外限流措施

2. **webhook 共享**: 一个 webhook 可能被多个服务引用,重建时如何处理?
   - **答**: 系统已通过 References 数组管理共享 webhook,重建时会正确处理引用关系

3. **Gerrit webhook 机制**: Gerrit 的 webhook 使用 remoteName 标识,与其他平台有差异,如何统一处理?
   - **答**: Gerrit 已纳入支持范围,通过 Service.GerritRemoteName 字段获取 remoteName(默认"origin"),在 checkWebhookExists 中针对 Gerrit 做特殊处理

## Alternatives considered

### 方案 A: 定时任务巡检

**优点**: 集中处理,可以发现所有失效的 webhook
**缺点**:
- 有时间延迟,可能几小时后才能修复
- 增加系统后台负担
- 批量 API 调用可能触发限流

**结论**: 不采用。实时修复更符合用户期望。

### 方案 B: 强制刷新模式

**优点**: 实现简单,用户可控
**缺点**:
- 需要用户主动操作,体验不佳
- 需要修改 UI 和 API

**结论**: 可作为补充方案,但不作为主要方案。

## References

- Webhook 表定义: `pkg/microservice/aslan/core/common/repository/models/webhook.go`
- 当前 webhook 处理: `pkg/microservice/aslan/core/common/service/service.go:752`
- 服务更新流程: `pkg/microservice/aslan/core/service/service/loader.go:76`
