# Zadig System Architecture Overview

## System Architecture

![Architecture_diagram](./Zadig-System-Architecture.svg)

## Main Components

User Interface:
- zadig-portal: Zadig web component
- Zadig Toolkit: vscode plugin

API Gateway:
- [Gloo Edge](https://github.com/solo-io/gloo): Zadig API gateway
- [OPA](https://github.com/open-policy-agent/opa): Authentication and authorization
- [Dex](https://github.com/dexidp/dex): Identity service for Zadig, acts as a portal to other identity providers like AD / LDAP / OAuth2 / GitHub / ..

Zadig Core:
- Aslan: main service for all business logic. Project, environment, service, workflow, build, system management are all in this service.
- Config:system configuration center.
- Workflow Runner:
  - warpdrive: workflow engine, manages reaper and predator
  - reaper: workflow runner. Used for building, testing tasks.
  - predator: workflow runner. Used for distribute tasks.
  - plugins: workflow plugins
    - Jenkins-plugin: workflow runner. Used as connector to trigger Jenkins job and retrieve job information.
- Cron: cronjob runner
- NSQ: message queue

Data Plane:
- MongoDB: database for business data.
- MySQL: dex configuration,database for user.

Kubernetes Cluster:
- Zadig business runs on standard K8s clusters from cloud vendors
