# Zadig 贡献指南

首先，非常感谢你使用 Zadig

Zadig 是一套分布式开源的持续部署系统，和其它 CI/CD 不同，Zadig 不仅可以提供高可用的 CI/CD 能力，同时内置很多面向不同技术场景的最佳实践。

Zadig 的成长离不开大家的支持。我们欢迎各类贡献，小到修改错别字、更新文档链接，大到负责从设计到研发的一个完整的功能。如果你愿意为其贡献代码或提供建议，
请阅读以下内容。

## 目录

- [Zadig 贡献指南](#zadig-贡献指南)
  - [目录](#目录)
  - [先决条件](#先决条件)
  - [环境设置](#环境设置)
  - [贡献方式 1 - 提交 issue](#贡献方式-1---提交-issue)
    - [Issue 提交后会被如何处理？](#issue-提交后会被如何处理)
  - [贡献方式 2 - 更改文档](#贡献方式-2---更改文档)
    - [简单的文档改动](#简单的文档改动)
    - [进阶的文档改动](#进阶的文档改动)
  - [贡献方式 3 - 提交代码](#贡献方式-3---提交代码)
    - [简单的代码改动](#简单的代码改动)
    - [进阶的代码改动](#进阶的代码改动)
    - [更新 API 文档](#更新-api-文档)
  - [贡献者资源](#贡献者资源)
    - [PR / Commit 指导](#pr--commit-指导)
    - [贡献者进阶之路](#贡献者进阶之路)
    - [如何获取帮助](#如何获取帮助)
    - [其它资源](#其它资源)

## 先决条件

每个 commit 都需要加上 Developer Certificate of Origin。这是一个很轻量的操作，具体可以见[这里](https://github.com/probot/dco#how-it-works)。

同时还想提醒大家遵循社区的 [Code of Conduct](CODE_OF_CONDUCT.md)，Zadig的良好发展离不开一个健康的社区，希望大家能一起维护。

## 环境设置

请先 fork 一份对应的仓库，不要直接在仓库下建分支。然后可以参考 [Zadig 开发流程](community/dev/contributor-workflow.md) 的介绍将 Zadig 环境搭建起来。

在决定提交 issue 或者提交任何更改之前，请查一下项目的 [open issues](https://github.com/koderover/zadig/issues)，避免重复。我们也准备
了几个 issue label 来帮助大家筛选：

1) 如果你想找一些适合上手的 issue，可以看下 [#good-first-issue](https://github.com/koderover/zadig/labels/good%20first%20issue)
有没有你感兴趣的。
2) 如果你在找更进阶点的贡献，可以看下 [#help-wanted](https://github.com/koderover/zadig/labels/help%20wanted) label 下的内容。
3) 如果你想找些 bug 来 fix，可以看下 [#bugs](https://github.com/koderover/zadig/labels/bug)。

## 贡献方式 1 - 提交 issue

**如果想上报的是和 security 相关的问题，请不要通过提交issue，而是发邮件到 contact@koderover.com。** 如果不是 security 相关的问题，请接着阅读这一章节。

贡献者提交 issue 的时候，有以下五种类型需要考虑：

1. [`documentation`](https://github.com/koderover/zadig/labels/documentation)
2. [`bug`](https://github.com/koderover/zadig/labels/bug)
3. [`feature request`](https://github.com/koderover/zadig/labels/feature%20request)
4. [`question`](https://github.com/koderover/zadig/labels/question)
5. [`enhancement`](https://github.com/koderover/zadig/labels/enhancement)

如果贡献者知道自己的 issue 是明确关于哪个或者哪几个服务的，也建议将服务相对应的 label 加上去：具体请[搜索我们带`service/`前缀的 label](https://github.com/koderover/zadig/labels?q=service%2F)；如果不确定的话可以放着，我们的 maintainer 会加上。

请首先检查下我们的 [open issues](https://github.com/koderover/zadig/issues)，确保不要提交重复的 issue。确认没有重复后，请选择上面类型之一的 label，并且按 issue 模板填好，尽可能详细的解释你的 issue —— 原则是要让没有你 context 的别人也能很容易的看懂。

### Issue 提交后会被如何处理？

我们项目的 [maintainer](GOVERNANCE.md#maintainers) 会监控所有的 issue，并做相应的处理：

1. 他们会再次确认新创建的 issue 是不是添加了上述五种 label 里正确的 label，如果不是的话他们会进行更新。
2. 他们同时也会决定是不是 accept issue，参见下一条。
3. 如果适用的话，他们可能会将以下四种新的 tag 加到 issue 上：
   1) [`duplicate`](https://github.com/koderover/zadig/labels/duplicate): 重复的 issue
   2) [`wonfix`](https://github.com/koderover/zadig/labels/wontfix)：决定不采取行动。maintainer 会说明不修复的具体原因，比如
      work as intended, obsolete, infeasible, out of scope
   3) [`good first issue`](https://github.com/koderover/zadig/labels/good%20first%20issue)：见上文，适合新人上手的 issue。
   4) [`good intermediate issue`](https://github.com/koderover/zadig/labels/good%20intermediate%20issue): 见上文，比较
      进阶的 issue，欢迎社区的贡献者来挑战。
4. issue 如果没有被关掉的话，现在就正式可以被认领（在issue上留言）了。
5. Maintainer 同时也会定期的检查和清理所有 issue，移除过期的 issue。

## 贡献方式 2 - 更改文档

### 简单的文档改动

对于非常简单的文档改动，比如改个错别字、更新个链接之类的，不需要走什么流程，直接建一个 PR 就行。我们的 Maintainer 会去 review。对于 PR 的具体要求，请参见
我们的 [PR / Commit 指导](#pr--commit-指导)。

### 进阶的文档改动

如果你想对文档做一些复杂点的改动，比如重塑文档的结构、增加一个新文档、添加几个章节等等，具体要求请遵循[进阶的代码改动](#进阶的代码改动)。你将会需要一个 issue 来跟踪，并且需要先提交你的改动方案并且方案被我们的 maintainer 通过。

## 贡献方式 3 - 提交代码

对于**任何**的代码改动，你**都需要有相应的 issue 来跟踪**：不管是现有的 issue 还是[创建一个新的 issue](#贡献方式-1---提交-issue)。

> 请在对应的 issue 下留言，表明你要 WORK ON 这个 issue，避免重复

### 简单的代码改动

对于简单的代码改动，我们的指导如下：

1. 你可以在对应的 issue 上简单描述下你的设计方案，收取反馈；当然如果你很自信改动非常简单直观并且你的改动基本不可能有什么问题的话，你也完全可以跳过这个步骤。
2. 在你fork的repository做相应的改动 - 具体的指导见 [Zadig 开发流程](community/dev/contributor-workflow.md)
3. 遵循我们 [PR / Commit 指导](#pr--commit-指导)，提交 PR，我们的 maintainer 会去 review。
4. 如果你添加或者修改了任何 `aslan` service 的 API, 你需要相应的[更新我们的API文档](#更新-api-文档).

### 进阶的代码改动

对于没那么直观或者稍微复杂点的代码改动：

1. 你需要先写一个设计文档：
   1) 复制一份我们的设计方案模板 [Zadig design template](community/rfc/yyyy-MM-dd-my-design-template.md)。
   2) 根据模板，描述下你想做什么和你的设计思路，以及相应的注意事项，比如数据库改动、向前兼容性考虑等。
   3) 文档尽量做到简洁、清楚。
2. 将你的设计方案按`yyyy-MM-dd-my-design-name.md`格式命名，并且放到 [community/rfc](community/rfc)目录下。为这个设计方案提交
   一个单独的 PR，我们的 maintainer 会去 review。
3. 如果这个设计方案被采纳并且 PR 被合并了，你就可以按照你设计文档中提供的方案做相应的修改了。
4. 我们强烈建议保持 PR 的原子性，如果你的项目可以被拆分成更细粒度的子任务，请尽量做拆分然后每一个子任务发一个单独的 PR。
5. 对于#4中提到的每一个子任务，参考上文非常轻量的 [简单的代码改动](#简单的代码改动) 指导。

### 更新 API 文档

如果你更改的不是 `aslan` service 的 API，那不需要考虑这个步骤。我们目前只对 `aslan` 维护API文档。

`aslan`的文档[参阅这里](https://os.koderover.com/api/spock/apidocs/index.html)：我们用[Swag](https://github.com/swaggo/swag)自动生成[Swagger](https://swagger.io/)文档；[Swag](https://github.com/swaggo/swag)会根据代码中API的注释（遵循[swag declarative API comments](https://github.com/swaggo/swag#declarative-comments-format)），自动生成文档.

所以如果你添加或者修改了任何 `aslan` 的API, 需要做以下几件事:

1. 遵循 [swag declarative API comments](https://github.com/swaggo/swag#declarative-comments-format) 给你的API加上合适的注释。
2. 使用以下命令来更新`aslan`的API文档:

```bash
cd [your root path of zadig]

make swag
```

更多细节参考 [Swag CLI](https://github.com/swaggo/swag#swag-cli)。

> 注意：如果你生成的doc/docs.go包含"github.com/alecthomas/template"(较早的swag版本)，请将它改成标准库"text/template"

3. 在你的测试环境下检查下生成的API文档是不是和你的期望一致。文档的相对路径是 `/api/aslan/apidocs/index.html`.

## 贡献者资源

### PR / Commit 指导

- 请遵循我们的 PR 模板，并且尽可能清晰、详细的描述你的 PR。
- 请保持每个 PR 的原子性，并且 PR 里的每条 commit message 都做到简洁、清晰、准确。
- 记得每条 commit 都要签名 [DCO](https://github.com/probot/dco#how-it-works)。
- 我们建议开发者尽早建立 PR，没必要等到全部工作都完成了再建 PR。只不过记得给 PR 的标题加上 WIP 前缀 -- "WIP: [title]"。当 PR 完成后可以 review 时再把
  前缀去掉。在那之后我们的 maintainer 就会去 review 了。
- 确保你的 PR 通过所有测试。
- 对于代码风格，请参考官方的 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)。
- 为你的 PR 添加上合适的 label(s)，具体怎么选择 label 和 issue 参见[贡献方式 1 - 提交 issue](#贡献方式-1---提交-issue)。
- 完整测试你改动后的代码--参考[如何调试代码](community/dev/contributor-workflow.md#4-调试)。

### 贡献者进阶之路

我们有清晰的开发者的晋级之路，请参见我们的 [GOVERNANCE](GOVERNANCE.md) 文档。

### 如何获取帮助

- 邮箱：contact@koderover.com
- 欢迎加入[slack channel](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

### 其它资源

- [项目的 maintainers](GOVERNANCE.md#maintainers)
