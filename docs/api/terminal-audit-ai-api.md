---
title: 终端会话 AI 审计 API
date: 2026-08-18 10:42:29
permalink: /cn/api/terminal-audit-ai/
---

# 终端会话 AI 审计 API

本文介绍终端会话 AI 审计的触发和结果查询接口。该能力读取已结束会话的命令记录与终端录制文件，调用系统默认大模型生成风险等级、审计摘要和风险项。

## 使用前须知

- 接口复用当前登录态鉴权，仅系统管理员可以调用。
- 只能审计已经结束且存在终端录制文件的会话；状态为 `running` 的会话不能触发审计。
- 调用前需要在系统中配置可用的默认大模型和对象存储。
- 触发接口当前为同步调用，首次请求会等待分析结束，最长等待时间约为 10 分钟。前端需要设置足够长的请求超时，并在请求期间展示加载状态。
- 同一会话同一时间只会运行一个审计任务。任务运行期间重复触发时，接口直接返回当前 `running` 结果。
- 对已有成功或失败结果再次触发，会重新执行审计并覆盖该会话最近一次结果。

## 接口总览

| 用途 | 方法 | 路径 |
| --- | --- | --- |
| 触发终端会话 AI 审计 | `POST` | `/api/aslan/system/terminalAudit/sessions/:sessionID/aiAudit` |
| 查询终端会话 AI 审计结果 | `GET` | `/api/aslan/system/terminalAudit/sessions/:sessionID/aiAudit` |

## 触发终端会话 AI 审计

对指定终端会话执行 AI 安全审计。接口不需要请求体。

**请求**

```http
POST /api/aslan/system/terminalAudit/sessions/:sessionID/aiAudit
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 是否必须 | 默认值 |
| --- | --- | --- | --- | --- |
| `sessionID` | string | 终端会话 ID | 是 | 无 |

**正常返回**

首次调用成功时返回最终审计结果：

```json
{
  "id": "68a3f26c7a35f741dd07b30a",
  "session_id": "terminal-session-01",
  "status": "succeeded",
  "risk_level": "high",
  "summary": "已审查 23 条终端命令，发现 2 项风险。",
  "findings": [
    {
      "seq": 3,
      "command": "curl https://example.com/install.sh | sh",
      "risk": "远程脚本执行",
      "reason": "远程脚本正文未被录制，无法确认实际执行内容。",
      "suggestion": "先下载脚本并完成内容审查和完整性校验，再执行脚本。"
    }
  ],
  "coverage": "partial",
  "model": "gpt-4o",
  "prompt_version": 1,
  "token_num": 3280,
  "analyzed_command_count": 23,
  "total_command_count": 23,
  "started_at": 1787011200,
  "finished_at": 1787011215,
  "created_at": 1787011200,
  "updated_at": 1787011215
}
```

任务正在运行时重复触发，返回当前运行状态：

```json
{
  "id": "68a3f26c7a35f741dd07b30a",
  "session_id": "terminal-session-01",
  "status": "running",
  "risk_level": "",
  "summary": "",
  "findings": [],
  "coverage": "",
  "model": "",
  "prompt_version": 0,
  "token_num": 0,
  "analyzed_command_count": 0,
  "total_command_count": 0,
  "started_at": 1787011200,
  "finished_at": 0,
  "created_at": 1787011200,
  "updated_at": 1787011200
}
```

::: warning
首次触发请求不会先返回 `running` 再转为后台执行，而是持续等待分析完成。如果页面卸载、前端主动取消请求或请求超时，分析上下文也会被取消。请求异常中断后，可调用结果查询接口确认最终状态。
:::

## 查询终端会话 AI 审计结果

查询指定终端会话最近一次保存的 AI 审计结果。

**请求**

```http
GET /api/aslan/system/terminalAudit/sessions/:sessionID/aiAudit
```

**路径参数说明**

| 参数名 | 类型 | 描述 | 是否必须 | 默认值 |
| --- | --- | --- | --- | --- |
| `sessionID` | string | 终端会话 ID | 是 | 无 |

**正常返回**

响应结构与触发接口相同。审计失败时返回示例：

```json
{
  "id": "68a3f26c7a35f741dd07b30a",
  "session_id": "terminal-session-01",
  "status": "failed",
  "risk_level": "",
  "summary": "",
  "findings": [],
  "coverage": "complete",
  "model": "gpt-4o",
  "prompt_version": 1,
  "token_num": 3280,
  "analyzed_command_count": 0,
  "total_command_count": 23,
  "error_message": "context canceled",
  "started_at": 1787011200,
  "finished_at": 1787011210,
  "created_at": 1787011200,
  "updated_at": 1787011210
}
```

从未触发过 AI 审计时，接口返回资源不存在错误。

## 返回说明

| 参数名 | 类型 | 描述 |
| --- | --- | --- |
| `id` | string | 审计结果 ID |
| `session_id` | string | 终端会话 ID |
| `status` | string | 审计状态，取值见[状态说明](#状态说明) |
| `risk_level` | string | 风险等级：`low`、`medium` 或 `high` |
| `summary` | string | 审计摘要 |
| `findings` | array | 风险项列表 |
| `coverage` | string | 证据覆盖范围：`complete` 或 `partial` |
| `model` | string | 本次审计使用的模型 |
| `prompt_version` | int | 审计 Prompt 版本 |
| `token_num` | int | 本次审计 Prompt 的 Token 估算数量 |
| `analyzed_command_count` | int64 | 完整纳入大模型输入的命令数量 |
| `total_command_count` | int64 | 会话记录的命令总数 |
| `error_message` | string | 失败原因，仅失败时返回 |
| `started_at` | int64 | 本次审计开始时间，Unix 秒级时间戳 |
| `finished_at` | int64 | 本次审计结束时间，Unix 秒级时间戳；运行中为 `0` |
| `created_at` | int64 | 该会话首次创建审计结果的时间，Unix 秒级时间戳 |
| `updated_at` | int64 | 审计结果更新时间，Unix 秒级时间戳 |

### Finding 参数说明

| 参数名 | 类型 | 描述 |
| --- | --- | --- |
| `seq` | int64 | 风险命令在会话中的序号 |
| `command` | string | 风险命令 |
| `risk` | string | 风险类型 |
| `reason` | string | 风险判断依据 |
| `suggestion` | string | 整改建议 |

## 状态说明

| 状态 | 描述 | 前端处理建议 |
| --- | --- | --- |
| `running` | 正在分析 | 展示加载状态，每 2～3 秒查询一次结果 |
| `succeeded` | 分析成功 | 展示风险等级、摘要、风险项和覆盖范围 |
| `failed` | 分析失败 | 展示 `error_message`，允许用户重新触发 |

## 证据覆盖范围

| 取值 | 描述 |
| --- | --- |
| `complete` | 会话证据未被截断，且不存在无法获取正文的执行内容 |
| `partial` | 部分证据未纳入审计，前端需要提示用户审计结果不代表完整会话 |

以下任一情况会使 `coverage` 为 `partial`：

- 会话命令超过 500 条，仅按时间顺序分析前 500 条。
- 证据超过 20 个分析分段，超出部分不会调用大模型。
- 终端录制数据达到保留上限并被截断。
- 会话包含远程脚本、管道脚本或其他无法获取脚本正文的执行方式。

## 前端联调流程

1. 进入终端会话详情页时，调用结果查询接口。
2. 返回资源不存在时，展示“开始分析”；返回已有结果时，按 `status` 渲染。
3. 用户点击“开始分析”后调用触发接口，并在请求期间保持页面和请求有效。
4. 触发接口返回 `running` 时，每 2～3 秒调用结果查询接口，直到状态变为 `succeeded` 或 `failed`。
5. `coverage=partial` 时，在审计结果区域展示“仅分析了部分会话证据”的提示。
