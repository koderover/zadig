# AI Review 点赞 / 点踩采集

当前只保存 MongoDB 数据，未增加页面和查询 API，供后续审查洞察面板使用。

## 统计口径和关联

- PR/MR 唯一键：`codehost_id + repo_owner + repo_name + pr`。GitLab `pr` 为 MR 的 IID，不是全局 ID。
- 累计同一 PR/MR 上所有已登记 AI Review 运行的总结和行内评论，按 reaction 数量计数，不按用户去重。
- 总结发布成功后保存 comment/note ID。GitHub 行内评论保存 review ID，采集该 review 下带 `<!-- zadig-ai-review -->` 标记的原始评论，排除回复。GitLab discussion 保存 note ID。
- GitLab Emoji Hook 使用 `source_host + project_id + merge_request.iid` 定位 MR，并确认 `awardable_id` 在 `comments` 中登记为 AI note。非 AI 评论、非 MR 评论、其他表情不会进入统计。
- 上线前未登记的历史评论不自动回填。发布总结失败时仍登记已经成功发布的行内评论。

## 调度和性能

| 行为 | 策略 |
| --- | --- |
| cron 扫描 | 每分钟调用 Aslan 内部 `/api/cron/cron/ai-review-feedback`，只选到期且未冻结的 PR/MR |
| GitHub | 首次登记后采集；完整采集成功后按 `AI_REVIEW_GITHUB_POLL_INTERVAL` 间隔采集，默认 **6 小时** |
| GitLab | Emoji Hook 根据 `event_type` 和表情名直接更新 MR 总数；平时不调用 API，不定时拉取；关闭或合并时全量校对一次 |
| close / merge | 强制全量最终采集，成功后冻结；reopen 恢复 GitLab webhook 更新或 GitHub 轮询 |
| 批量规模 | 每次扫描最多 10 个 PR/MR，总同步 context 45 秒；单进程禁止重叠，多副本使用 5 分钟数据库租约 |
| API 分页 | 每页 100；GitHub 按 review 批次，GitLab 仅最终校对时按已登记 note 拉取 |
| 写入 | 登记目标时每批最多 100 个原子追加，同一 `kind + comment_id` 不会重复登记或覆盖快照；采集后的嵌入评论快照及时保存，下一次采集时跳过本轮已经完成的目标 |
| 失败 | 保留旧计数，记录错误，等待下一个正常采集周期；GitHub 使用配置间隔，GitLab 最终校对失败时在下一分钟扫描继续校对，不额外安排限流重试 |

配置通过 Aslan 配置层的 `config.AIReviewGitHubPollInterval()` 使用 `viper.GetString` 读取，复用已有的 `viper.AutomaticEnv()` 初始化。在 **Aslan 服务**中设置环境变量，例如：

```yaml
env:
  - name: AI_REVIEW_GITHUB_POLL_INTERVAL
    value: "12h"
```

使用 Go duration 格式，支持 `6h`、`30m`、`1h30m` 等正数时长；未设置、空值、无效值、零或负值均回退为 **6 小时**。部署修改环境变量后需重启 Aslan；新间隔在下一次完整采集成功后用于计算下次采集时间，数据库中已经排定的采集时间不会自动重排。cron 仍每分钟扫描到期记录，因此实际采集时效受扫描频率和处理积压影响。

配置值是正常完整采集的间隔；配额耗尽、接口故障或处理积压会延后，不能保证外部 API 故障期间的更新时效。

GitLab webhook 在验签后直接更新 `ai_review_feedback` 中的 MR 总数，不调用外部 API。动作读取顶层 `event_type`：`award` 新增、`revoke` 撤销。按 `object_attributes.id` 保存每条表情记录的状态，重复投递不重复加减；先收到撤销时保留撤销状态，之后迟到的新增不会计入。计数、表情状态和 revision 在同一个 MongoDB 文档中原子写入，并发事件遇到 revision 冲突时重新读取后更新。PR 文档的 `comments` 用于匹配 AI 评论，并保存 API 采集快照及进度；GitLab 的评论快照在最终校对时更新，日常数量以 PR 文档顶层 `up/down` 为准。

关闭/合并事件设置 `final_sync`，每分钟任务只扫描待最终校对的 GitLab MR；成功后清除此标记并冻结。校对过程中若有新事件，revision 冲突会保留待校对状态。仅依靠 webhook 无法补回漏投的历史变化，关闭时的最终 API 校对会修正总数。

## MongoDB

| 集合 | 用途 |
| --- | --- |
| `ai_review_feedback` | 每个 PR/MR 一条，保存总数、调度和租约、GitLab 去重状态，以及嵌入的 `comments` 目标列表 |

`comments` 每项包含 `kind`、`comment_id`、`review_id`、`up/down` 快照、`synced_at` 和 `dirty_at`。GitHub 总结使用 comment ID，行内审查使用 review ID；已完成的目标在当前采集周期内可以跳过。追加新评论和 webhook 更新都会增加 PR revision，阻止旧任务覆盖新数据。

`initconfig` 只登记 `ai_review_feedback` 的唯一键、到期扫描索引和嵌入 note 查询索引。按要求不迁移旧数据；运行时不读写旧 `ai_review_feedback_comment` 集合，旧记录没有 `comments` 时不进入采集，也不会被清零。新发布的 AI 审查会直接登记到 PR 文档。

## GitLab webhook

GitHub App client 按实际使用的 App 和仓库所属用户/组织复用，不设置固定数量上限或有效期；SDK 自动刷新 token。下次读取配置发现 App ID、密钥或代理配置变化时清除旧客户端；GitHub 请求失败时清除对应客户端，下一个正常采集周期重新查询 installation 并创建客户端。GitLab 最终校对结束会关闭其空闲 HTTP 连接。

新建或更新 AI 审查时，直接根据选定的 GitLab 仓库，复用现有 `ProcessWebhook` 和 webhook controller 检查并补齐 webhook，不依赖自动触发开关或触发器列表。已有 URL 与 Zadig webhook 地址完全一致的 hook 时补上 Emoji events，保留其他配置；没有时创建包含 Push、Merge request、Tag push、Emoji events 和 Zadig 验签 secret 的 hook，并保存 hook ID、维护原有引用关系。GitLab 创建入口统一复用此逻辑，其他回调地址的 hook 不会被修改。外部检查使用 2 分钟 context，上层 webhook 任务等待 3 分钟；检查失败通过原有配置保存流程返回错误，已有 webhook 数据库记录不会被删除。关闭自动触发后仍保留 AI 审查反馈所需的 webhook；更换仓库或删除审查配置时移除旧引用，按原有规则处理共享 webhook。登记 PR/MR 评论和定时采集均不检查或创建 webhook。

Emoji Hook 需支持 GitLab 的 [Emoji events](https://docs.gitlab.com/user/project/integrations/webhook_events/#emoji-events)，最终校对需具备读取 note award emoji 的权限。最终采集完成并冻结后的新增 reaction 不纳入统计。

## 验证

单元测试覆盖webhook 加减、重复和乱序投递、GitLab 最终校对及 GitHub 分页、快照保留和正常采集间隔。

```sh
go test ./pkg/microservice/aslan/core/common/service/reviewfeedback ./pkg/microservice/aslan/core/common/service/scmnotify ./pkg/microservice/aslan/core/common/repository/mongodb -run 'TestFeedback|TestGitHub|TestGitLab|Test.*AIReview' -count=1
```

真实环境验收：发布一个有总结和行内评论的 PR/MR，检查关联记录；加/撤销两种表情并检查聚合；重复投递 GitLab 事件确认总数不会重复累加；关闭后验证最终刷新和冻结；重开后验证恢复；模拟外部限流确认保留旧计数并等待下一个正常采集周期。GitHub 轮询可在测试环境将该 PR 的 `next_sync_at`、`full_sync_at` 改为当前时间验证，无需修改生产的采集间隔。
