<p><a href="https://github.com/koderover/zadig-doc" target="_blank" rel="noopener noreferrer"><img height="50" src="https://docs.koderover.com/zadig/img/zadig.png" alt="Zadig logo"></a></p>

<h3 align="left">AI 驱动的开发者云原生 DevOps 平台</h3>

<span align="left">


[![LICENSE](https://img.shields.io/github/license/koderover/zadig.svg)](https://github.com/koderover/zadig/blob/main/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
⁣[![Go Report Card](https://goreportcard.com/badge/github.com/koderover/zadig)](https://goreportcard.com/report/github.com/koderover/zadig)
![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/koderover/zadig?include_prereleases)
[!["Join us on Slack"](https://img.shields.io/badge/join-us%20on%20slack-gray.svg?longCache=true&logo=slack&colorB=brightgreen)](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

[![官方网站](<https://img.shields.io/badge/官方网站-rgb(24,24,24)?style=for-the-badge>)](https://www.koderover.com/?utm_source=github&utm_medium=zadig_readme)
[![在线试用](<https://img.shields.io/badge/在线试用-rgb(255,41,104)?style=for-the-badge>)](https://www.koderover.com/trial/?utm_source=github&utm_medium=zadig_readme)

</span>

<div align="left">

**[English](./README.md) | 简体中文**

</div>

## 目录

- [目录](#目录)
- [Zadig 介绍](#zadig-介绍)
- [快速上手](#快速上手)
  - [快速使用](#快速使用)
  - [训练营](#训练营)
  - [快速开发](#快速开发)
- [获取帮助](#获取帮助)
- [代码许可](#代码许可)

## Zadig 介绍

Zadig 由 KodeRover 自主研发，是基于 Kubernetes 与 AI 大模型的开源云原生 DevOps 平台，致力于帮助企业实现产研数字化转型。
核心能力覆盖灵活可扩展的工作流、多种发布策略编排、一键安全审核、AI 环境巡检与效能诊断、定制化企业级 XOps 敏捷看板，并支持深度集成企业平台、项目模板化批量纳管数千服务。
Zadig 全面融入 AI 代码审查、AI 发布风险评估、AI 任务编排及智能体管理等智能化能力，让代码质量防线前移、发布决策更精准、交付流程更自动，真正使工程师成为创新引擎，为数字经济的持续创新提供坚实基座。

我们的愿景：`工程师 + Zadig = 商业上的成功`

业务架构介绍：

![业务架构图](./Zadig-Business-Architecture-zh.jpg)

想了解更多系统架构信息，参考 [系统架构简介](System-Architecture-Overview-zh-CN.md).

产品特性介绍：

<details>
  <summary><b>AI 驱动的全流程提效</b></summary>
  深度集成 AI 大模型，覆盖开发、发布、运维全链路：AI 代码审查精准识别代码缺陷与安全漏洞；AI 发布风险评估智能分析变更影响，辅助安全可控发布；AI 任务编排将企业智能体无缝嵌入研发流程。同时提供 AI 效能诊断与环境巡检，精准定位效能瓶颈，定期预警环境风险。
  </details>

<details>
  <summary><b>灵活易用的高并发工作流</b></summary>
  简单配置，可自动生成高并发工作流，多个微服务可并行构建、并行部署、并行测试，大大提升代码验证效率。自定义的工作流步骤，配合人工审核，灵活且可控的保障业务交付质量。
  </details>

<details>
  <summary><b>面向开发者的云原生环境</b></summary>
  分钟级创建或复制一套完整的隔离环境，应对频繁的业务变更和产品迭代。基于全量基准环境，快速为开发者提供一套独立的自测环境。一键托管集群资源即可轻松调试已有服务，验证业务代码。
  </details>

<details>
  <summary><b>高效协同的测试管理</b></summary>
  便捷对接 Jmeter、Pytest 等主流测试框架，跨项目管理和沉淀 UI、API、E2E 测试用例资产。通过工作流，向开发者提供前置测试验证能力。通过持续测试和质量分析，充分释放测试价值。
  </details>

<details>
  <summary><b>强大免运维的模板库</b></summary>
  跨项目共享 K8s YAML 模板、Helm Chart 模板、构建模板等，实现配置的统一化管理。基于一套模板可创建数百微服务，开发工程师少量配置可自助使用，大幅降低运维管理负担。
  </details>

<details>
  <summary><b>安全可靠的发布管理</b></summary>
  自定义工作流打通人、流程、内外部系统合规审批，支持灵活编排蓝绿、金丝雀、分批次灰度、Istio 等发布策略。通过多集群、多项目视角呈现生产环境的状态，实现发布过程的透明可靠。
  </details>

<details>
  <summary><b>客观精确的效能洞察</b></summary>
  全面了解系统运行状态，包括集群、项目、环境、工作流，关键过程通过率等数据概览。提供项目维度的构建、测试、部署等客观的效能度量数据，精准分析研发效能短板，促进稳步提升。
  </details>

## 快速上手

### 快速使用

请参阅 [快速入门](https://docs.koderover.com/zadig/quick-start/introduction/)

### 训练营

Zadig [训练营](https://github.com/koderover/zadig-bootcamp)主要是为开发者提供实践小技巧、最佳实践案例的搭建、典型应用场景的演示等，以便快速获得持续交付最佳解决方案。可以直接进入 [教程](https://koderover.com/tutorials) 一步步实践和尝试。

### 快速开发

请阅读完整的 [Zadig 贡献指南](CONTRIBUTING-zh-CN.md)，该包含参与贡献的方式、流程、格式、如何部署、哪里可以获取帮助等。

如果你已经阅读过上面的文档，想快速进入开发状态的话，可以直接进入 [Zadig 开发流程](community/dev/contributor-workflow.md)。

## 获取帮助

- 更详细的使用说明，见 [文档站](https://docs.koderover.com?type=zadig)
- 如果发现了bug或者功能需求，[欢迎提交issue](CONTRIBUTING-zh-CN.md#贡献方式-1---提交issue)
- 邮箱：contact@koderover.com

## 代码许可

[Apache 2.0 License](./LICENSE)
