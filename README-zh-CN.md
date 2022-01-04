<p><a href="https://github.com/koderover/zadig-doc" target="_blank" rel="noopener noreferrer"><img height="50" src="https://docs.koderover.com/zadig/img/zadig.png" alt="Zadig logo"></a></p>

<h3 align="left">开源分布式持续交付产品</h3>

<span align="left">

[![Zadig CI](https://os.koderover.com/api/collie/api/badge?pipelineName=zadig-ci/zadig-ci&source=github&repoFullName=koderover/zadig&branch=main&eventType=push)](https://os.koderover.com/v1/projects/detail/zadig-ci/pipelines/freestyle/home/zadig-ci/608824fef341de000137317d?rightbar=step)
[![LICENSE](https://img.shields.io/github/license/koderover/zadig.svg)](https://github.com/koderover/zadig/blob/main/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
⁣[![Go Report Card](https://goreportcard.com/badge/github.com/koderover/zadig)](https://goreportcard.com/report/github.com/koderover/zadig)
![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/koderover/zadig?include_prereleases)
[!["Join us on Slack"](https://img.shields.io/badge/join-us%20on%20slack-gray.svg?longCache=true&logo=slack&colorB=brightgreen)](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

</span>

<div align="left">

**[English](./README.md) | 简体中文**

</div>

## 目录

- [Zadig](#zadig)
  - [目录](#目录)
  - [Zadig 介绍](#zadig-介绍)
  - [快速上手](#快速上手)
    - [快速使用](#快速使用)
    - [快速开发](#快速开发)
  - [获取帮助](#获取帮助)
  - [更多文档](#更多文档)
  - [代码许可](#代码许可)

## Zadig 介绍

Zadig 是一款面向开发者设计的云原生持续交付(Continuous Delivery)产品，具备高可用 CI/CD 能力，提供云原生运行环境，支持开发者本地联调、微服务并行构建和部署、集成测试等。Zadig 不改变现有流程，无缝集成 Github/Gitlab、Jenkins、多家云厂商等，运维成本极低。我们的目标是通过云原生技术的运用和工程产品赋能，打造极致、高效、愉悦的开发者工作体验，让工程师成为企业创新的核心引擎。


我们的愿景：`工程师 + Zadig = 商业上的成功`

业务架构介绍：

![业务架构图](./Zadig-Business-Architecture-zh.jpg)

想了解更多系统架构信息，参考 [系统架构简介](System-Architecture-Overview-zh-CN.md).

产品特性介绍：

<details>
  <summary><b>高并发的工作流</b></summary>
  基于云原生设计，经过简单配置，系统自动生成工作流，实现多服务高并发执行构建部署测试任务，以解决微服务架构下带来的多服务构建部署效率低下问题。
  </details>

<details>
  <summary><b>以服务为核心的集成环境</b></summary>
  一套服务配置，分钟级创建多套数据隔离的测试环境。为开发者进行日常调试、为测试人员做集成测试、为产品经理对外 Demo 提供强力支撑。

  对于现有的环境无需担心迁移成本，一键托管，轻松浏览、调试环境中的所有服务。
  </details>

<details>
  <summary><b>无侵入的自动化测试</b></summary>
  便捷且无侵入的对接已有自动化测试框架，通过 GitHub/GitLab Webhook 自动构建、部署及测试。

  通过办公通讯机器人为开发者提供第一时间质量反馈，精准高效。有效落地“测试左移”工程实践，让测试价值得到体现。
  </details>

  <details>
  <summary><b>开发本地联调 CLI/IDE Plugin 插件</b></summary>
  开发本地编辑完代码，一键进行本地代码构建，部署到联调环境，无需再陷入复杂且繁琐的工作流程。解放工程师双手，去创造更多产品价值。
  </details>


## 快速上手

### 快速使用

请参阅 [快速入门](https://docs.koderover.com/zadig/quick-start/try-out-install/)

### 训练营

Zadig [训练营](https://github.com/koderover/zadig-bootcamp)主要是为开发者提供实践小技巧、最佳实践案例的搭建、典型应用场景的演示等，以便快速获得持续交付最佳解决方案。可以直接进入 [教程](https://www.koderover.com/tutorials) 一步步实践和尝试。

### 快速开发

请阅读完整的 [Zadig 贡献指南](CONTRIBUTING-zh-CN.md)，该包含参与贡献的方式、流程、格式、如何部署、哪里可以获取帮助等。

如果你已经阅读过上面的文档，想快速进入开发状态的话，可以直接进入 [Zadig 开发流程](community/dev/contributor-workflow.md)。

## 获取帮助

- 更详细的使用说明，见 [文档站](https://docs.koderover.com/zadig)
- 如果发现了bug或者功能需求，[欢迎提交issue](CONTRIBUTING-zh-CN.md#贡献方式-1---提交issue)
- 邮箱：contact@koderover.com
- 欢迎加入 [slack channel](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

## 代码许可

[Apache 2.0 License](./LICENSE)
