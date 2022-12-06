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

Zadig 是 KodeRover 公司基于 Kubernetes 自主设计、研发的开源分布式持续交付 (Continuous Delivery) 产品，具备灵活易用的高并发工作流、面向开发者的云原生环境、高效协同的测试管理、强大免运维的模板库、客观精确的效能洞察以及云原生 IDE 插件等重要特性，为工程师提供统一的协作平面。Zadig 内置了 K8s YAML、Helm Chart、主机等复杂场景最佳实践，适用大规模微服务、高频高质量交付等场景。我们的目标是通过云原生技术的运用和工程产品赋能，打造极致、高效、愉悦的开发者工作体验，让工程师成为企业创新的核心引擎。


我们的愿景：`工程师 + Zadig = 商业上的成功`

业务架构介绍：

![业务架构图](./Zadig-Business-Architecture-zh.jpg)

想了解更多系统架构信息，参考 [系统架构简介](System-Architecture-Overview-zh-CN.md).

产品特性介绍：

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
  <summary><b>客观精确的效能洞察</b></summary>
  全面了解系统运行状态，包括集群、项目、环境、工作流，关键过程通过率等数据概览。提供项目维度的构建、测试、部署等客观的效能度量数据，精准分析研发效能短板，促进稳步提升。
  </details>

  <details>
  <summary><b>云原生 IDE 插件</b></summary>
  开发者无需平台切换，在 VScode IDE 中即可获得 Zadig 产品核心能力。编写代码后，无需打包镜像，即可一键热部署到自测环境，快速完成自测、联调和集成验证，开发效率倍增。
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
