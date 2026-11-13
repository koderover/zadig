package service

import (
	"encoding/json"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// 持久化副本隐藏工具参数中的变量值，原事件仍供当前工具执行使用。
func privateEventSnapshot(original *event.Event) *event.Event {
	copy := original.Clone()
	copy.ID, copy.Version, copy.FilterKey = original.ID, original.Version, original.FilterKey
	if copy.Response != nil {
		for i := range copy.Choices {
			copy.Choices[i].Message.ToolCalls = append([]model.ToolCall(nil), copy.Choices[i].Message.ToolCalls...)
			for j := range copy.Choices[i].Message.ToolCalls {
				call := &copy.Choices[i].Message.ToolCalls[j]
				if call.Function.Name == "project_onboarding" || call.Function.Name == "service_onboarding" {
					call.Function.Arguments = maskToolVariables(call.Function.Arguments)
				}
			}
		}
	}
	if raw := copy.Extensions["trpc_agent.tool_call_args"]; len(raw) > 0 {
		var args map[string]string
		if json.Unmarshal(raw, &args) == nil {
			for id, value := range args {
				args[id] = string(maskToolVariables([]byte(value)))
			}
			copy.Extensions["trpc_agent.tool_call_args"], _ = json.Marshal(args)
		}
	}
	return copy
}

func maskToolVariables(raw []byte) []byte {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var mask func(any)
	mask = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if key == "variables" {
					if values, ok := item.(map[string]any); ok {
						for name := range values {
							values[name] = "[已隐藏]"
						}
					}
				} else {
					mask(item)
				}
			}
		case []any:
			for _, item := range v {
				mask(item)
			}
		}
	}
	mask(value)
	result, _ := json.Marshal(value)
	return result
}
