# Zadig System Architecture Overview

## System Architecture

![Architecture_diagram](./Zadig-System-Architecture.svg)

## Main Components

User Interface：
- zadig-portal：zadig web component
- kodespace：zadig command line tools
- Zadig Toolkit：vscode plugin

API Gateway：
- Gloo Edge：Zadig API gateway
- OPA：Authentication and authorization
- User：User management, token generation
- Dex：Identity service for Zadig, acts as a portal to other identity providers like AD, LDAP, OAuth2, GitHub, ...

Zadig Core：
- Picket：backend for frontend service.
- Policy:  data source of OPA and policy registration center.
- Aslan：main service for all business logic. Project, environment, service, workflow, build, system management are all in this service.
- Config: system configuration center.

- Workflow Runner：
  - warpdrive：workflow engine, manages reaper and predator
  - reaper: workflow runner. Used for building, testing tasks.
  - predator：workflow runner. Used for distribute tasks.
  - plugins：workflow plugins
    - Jenkins-plugin: workflow runner. Used as connector to trigger Jenkins job and retrieve job information.
- cron：cronjob runner
- nsq：message queue

Data Plane：
- mongodb：database for business data.
- mysql：dex configuration,database for user.

Kubernetes Cluster：
- zadig business runs on standard K8s clusters from cloud vendors
