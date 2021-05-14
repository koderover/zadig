# Zadig

<h3 align="left">Developer-oriented Continuous Delivery Product</h3>

<span align="left">

[![Zadig CI](https://os.koderover.com/api/collie/api/badge?pipelineName=zadig-ci/zadig-ci&source=github&repoFullName=koderover/Zadig&branch=main&eventType=push)](https://os.koderover.com/v1/projects/detail/zadig-ci/pipelines/freestyle/home/zadig-ci/608824fef341de000137317d?rightbar=step)
[![LICENSE](https://img.shields.io/github/license/koderover/zadig.svg)](https://github.com/koderover/zadig/blob/main/LICENSE)
[![Language](https://img.shields.io/badge/Language-Go-blue.svg)](https://golang.org/)
![GitHub all releases](https://img.shields.io/github/downloads/koderover/Zadig/total)

</span>

<div align="left">

**English | [简体中文](./README-zh-CN.md)**

</div>

## Table of Contents

- [Zadig](#zadig)
  - [Table of Contents](#table-of-contents)
  - [What is Zadig](#what-is-zadig)
  - [Quick start](#quick-start)
    - [How to use?](#how-to-use)
    - [How to make contribution?](#how-to-make-contribution)
  - [Getting help](#getting-help)
  - [More resources](#more-resources)
  - [License](#license)

## What is Zadig

Zadig is an open-source, distributed, cloud-native CD (Continuous Delivery) product designed for developers. Zadig not only provides high-availability CI/CD capabilities, but also provides cloud-native operating environments, supports developers' local debugging, parallel build and deployment of microservices, integration testing, etc. .

Zadig is non-invasive, it does not exclude any of your existing development process. Instead it can easily integrate with Github/Gitlab, Jenkins and many other cloud vendors in a seamingless way. We strive for the 10x optimal developer experience with the lowest maintenance cost possible.

> Our vision is: Developer + Zadig = Business success

- **High Concurrency**

Based on cloud-native design, through simple configuration, the system automatically generates workflows to achieve high concurrent execution for continuous delivery relevant tasks such as building, testing and deployment, across multiple services. It significantly improves the efficiency of multi-services deployment in microservice architecture.

- **Service-oriented Environment**

With just one set of service configuration, multiple encapsulated environments will be provided automatically within minutes, empowering independent environments for developers, QAs and product managers.

Minimum to none migration cost of existing environments -- just hosting with one click, the system allows browsing and adjusting all the services at your fingertips.

- **Non-intrusive Testing Automation**

Zadig can easily and non-intrusively embed existing testing automation frameworks, and achieve continuous building, testing and deployment via GitHub/GitLab Webhook.

It also integrates with productivity bots to provide instant quality report, which effectively applies shift-left testing best practices.

- **Convenient Development CLI**

Zadig also provides a convenient toolkit with development commandline interface which allows compiling, building and deploying the changes to dev environment with one command. It enables collaborated debugging and testing with minimum manual toil, reduces cognitive load and allows teams to focus more on business.

## Quick start

### How to use?

Please follow <<[Quick Start](https://docs.koderover.com/zadig/quick-start/try-out-install)>>

### How to make contribution?

Please check out [our contributing guideline](CONTRIBUTING.md).

## Getting help

- More about Zadig, see [here](https://github.com/koderover/Zadig-doc)
- Submit bugs or feature requests following [contributing instructions](CONTRIBUTING.md#contribution-option-1---reporting-an-issue)
- Email：contact@koderover.com
- [Slack channel](https://join.slack.com/t/zadig-workspace/shared_invite/zt-qedvct1t-mQUf2eyTRkoVCc_RWKKgxw)

## License

[Apache 2.0 License](./LICENSE)
