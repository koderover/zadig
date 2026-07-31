<p><a href="https://github.com/koderover/zadig-doc" target="_blank" rel="noopener noreferrer"><img height="50" src="https://docs.koderover.com/zadig/img/zadig.png" alt="Zadig logo"></a></p>

<h3 align="left">AI-powered, cloud-native, distributed DevOps platform designed for developers</h3>

<span align="left">

[![Codacy Badge](https://api.codacy.com/project/badge/Grade/2276bb4cce9348ceb8ddeeea5803ed57)](https://app.codacy.com/gh/koderover/zadig?utm_source=github.com&utm_medium=referral&utm_content=koderover/zadig&utm_campaign=Badge_Grade_Settings)
[![LICENSE](https://img.shields.io/github/license/koderover/zadig.svg)](https://github.com/koderover/zadig/blob/main/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
⁣[![Go Report Card](https://goreportcard.com/badge/github.com/koderover/zadig)](https://goreportcard.com/report/github.com/koderover/zadig)
![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/koderover/zadig?include_prereleases)
[!["Join us on Slack"](https://img.shields.io/badge/join-us%20on%20slack-gray.svg?longCache=true&logo=slack&colorB=brightgreen)](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

[![Official Website](<https://img.shields.io/badge/-Visit%20the%20Official%20Website%20%E2%86%92-rgb(24,24,24)?style=for-the-badge>)](https://www.koderover.com/?utm_source=github&utm_medium=zadig_readme)
[![Playground](<https://img.shields.io/badge/-Try%20It%20Online%20%E2%86%92-rgb(255,41,104)?style=for-the-badge>)](https://www.koderover.com/trial/?utm_source=github&utm_medium=zadig_readme)


</span>

<div align="left">

**English | [简体中文](./README-zh-CN.md)**

</div>

## Table of Contents

- [Table of Contents](#table-of-contents)
- [What is Zadig](#what-is-zadig)
- [Quick start](#quick-start)
  - [How to use?](#how-to-use)
  - [Bootcamps](#bootcamps)
  - [How to make contribution?](#how-to-make-contribution)
- [Getting help](#getting-help)
- [License](#license)

## What is Zadig

Zadig is an open-source, cloud-native DevOps platform developed by KodeRover, built on Kubernetes and AI large language models, and dedicated to helping enterprises achieve digital transformation in product R&D.
Its core capabilities cover flexible and extensible workflows, multiple release strategy orchestrations, one-click security audits, AI environment inspection and efficiency diagnosis, customized enterprise-level XOps agile dashboards, and deep integration with enterprise platforms, plus project templates for managing thousands of services in batches.
Zadig fully embeds intelligent capabilities such as AI code review, AI release risk assessment, AI task orchestration, and agent management, shifting quality gates left, making release decisions more precise, and delivery more automated—empowering engineers as engines of innovation and providing a solid foundation for continuous innovation in the digital economy.

> Our vision is: Developer + Zadig = Business success


The business architecture is as follows:

![Business Architecture](./Zadig-Business-Architecture.jpg)

For more details about system architecture, see [Zadig System Architecture Overview](System-Architecture-Overview.md).

The Highlighted Features:

<details>
  <summary><b>AI-Driven End-to-End Efficiency</b></summary>
  Deeply integrated with AI large language models across development, release, and operations: AI code review precisely identifies defects and security vulnerabilities; AI release risk assessment intelligently analyzes change impact to support safer releases; AI task orchestration seamlessly embeds enterprise agents into the R&D process. It also provides AI efficiency diagnosis and environment inspection to pinpoint bottlenecks and regularly warn about environmental risks.
  </details>

<details>
  <summary><b>High Concurrency</b></summary>
  Based on cloud-native design, through simple configuration, the system automatically generates workflows to achieve high concurrent execution for continuous delivery relevant tasks such as building, testing and deployment, across multiple services. It significantly improves the efficiency of multi-services deployment in microservice architecture.
  </details>

<details>
  <summary><b>Service-oriented Environment</b></summary>
  With just one set of service configuration, multiple encapsulated environments will be provided automatically within minutes, empowering independent environments for developers, QAs and product managers.

  Minimum to none migration cost of existing environments -- just hosting with one click, the system allows browsing and adjusting all the services at your fingertips.
  </details>

<details>
  <summary><b>Non-intrusive Testing Automation</b></summary>
  Zadig can easily and non-intrusively embed existing testing automation frameworks, and achieve continuous building, testing and deployment via GitHub/GitLab Webhook.

  It also integrates with productivity bots to provide instant quality report, which effectively applies shift-left testing best practices.
  </details>

<details>
  <summary><b>Powerful Template Library</b></summary>
  Shared K8s YAML templates, Helm Chart templates, build templates, etc., across projects to achieve unified configuration management. Based on a set of templates, hundreds of microservices can be created, and with minimal configuration by development engineers, self-service usage is enabled, significantly reducing the burden of operations and maintenance management.
  </details>

<details>
  <summary><b>Secure and Reliable Release Management</b></summary>
  Customized workflows that integrate engineer, process, and internal and external system compliance approvals, supporting flexible orchestration of deployment strategies such as blue-green deployment, canary release, phased gray release, and Istio release. Presenting the state of the production environment from a multi-cluster, multi-project perspective, it achieves transparency and reliability in the release process.
  </details>

<details>
  <summary><b>Objective and Accurate Performance Insights</b></summary>
  Comprehensive understanding of the system's operational status, including data overviews on clusters, projects, environments, workflows, and key process pass rates. It provides objective performance metrics data for various aspects like builds, tests, and deployments at the project level, enabling precise analysis of development efficiency bottlenecks and promoting steady improvement.
  </details>


## Quick start

### How to use?

Please follow [Quick Start](https://docs.koderover.com/zadig/quick-start/introduction/)

### Bootcamps

With many hands-on tips, case studies and demos of different application types, Zadig [bootcamp](https://github.com/koderover/zadig-bootcamp) gives developers easy to follow best practices in a cloud native CI/CD solution

Just follow [Tutorials](https://koderover.com/tutorials) to try it step by step.

### How to make contribution?

Please check out [our contributing guideline](CONTRIBUTING.md).

## Getting help

- More about Zadig, see [here](https://docs.koderover.com?type=zadig)
- Submit bugs or feature requests following [contributing instructions](CONTRIBUTING.md#contribution-option-1---reporting-an-issue)
- Email：contact@koderover.com


## License

[Apache 2.0 License](./LICENSE)
