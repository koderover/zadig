package service

import "trpc.group/trpc-go/trpc-agent-go/tool"

// The SDK reflects interface{} as object. Defaults must instead accept raw JSON
// primitives/arrays, and each tool should expose only the fields it actually uses.
func uiToolInputSchema(kind string) *tool.Schema {
	str := func() *tool.Schema { return &tool.Schema{Type: "string"} }
	boolean := func() *tool.Schema { return &tool.Schema{Type: "boolean"} }
	number := func() *tool.Schema { return &tool.Schema{Type: "number"} }
	array := func(item *tool.Schema) *tool.Schema { return &tool.Schema{Type: "array", Items: item} }
	object := func(properties map[string]*tool.Schema, required ...string) *tool.Schema {
		return &tool.Schema{Type: "object", Properties: properties, Required: required, AdditionalProperties: false}
	}
	enum := func(values ...string) *tool.Schema {
		s := str()
		for _, value := range values {
			s.Enum = append(s.Enum, value)
		}
		return s
	}
	option := object(map[string]*tool.Schema{"label": str(), "value": str()}, "label", "value")
	target := object(map[string]*tool.Schema{"project_key": str(), "env_name": str(), "service_name": str(), "production": boolean()}, "project_key")
	action := object(map[string]*tool.Schema{"id": str(), "label": str(), "type": enum("submit", "prefill", "navigate"), "prompt": str(), "target": target}, "id", "label", "type")
	properties := map[string]*tool.Schema{"title": str()}
	required := []string{"title"}
	switch kind {
	case "form":
		field := object(map[string]*tool.Schema{
			"id": str(), "label": str(), "type": enum("text", "textarea", "number", "select", "radio", "checkbox", "switch"), "required": boolean(),
			"default": {Description: "Raw JSON value matching the field type: number 0, boolean false, string, or array. Never wrap in an object."},
			"min":     number(), "max": number(), "min_length": {Type: "integer"}, "max_length": {Type: "integer"}, "options": array(option),
		}, "id", "label", "type")
		properties["data"] = object(map[string]*tool.Schema{"fields": array(field)}, "fields")
		properties["actions"] = array(action)
		required = append(required, "data")
	case "question":
		question := object(map[string]*tool.Schema{
			"id": str(), "title": str(), "description": str(), "type": enum("single", "multiple", "text"),
			"required": boolean(), "options": array(option), "allow_other": boolean(),
		}, "id", "title", "type")
		question.Description = "For single/multiple use 2-8 distinct options; text has no options or allow_other. Keep titles concise."
		questions := array(question)
		questions.Description = "One to five short questions."
		properties["data"] = object(map[string]*tool.Schema{"questions": questions}, "questions")
		required = append(required, "data")
	case "resource_selector":
		field := object(map[string]*tool.Schema{"id": str(), "label": str(), "resource_type": enum("project", "environment", "service"), "parent_field_id": str(), "production": boolean(), "multiple": boolean(), "required": boolean()}, "id", "label", "resource_type")
		properties["data"] = object(map[string]*tool.Schema{"fields": array(field)}, "fields")
		properties["actions"] = array(action)
		required = append(required, "data")
	case "actions":
		properties["actions"] = array(action)
		required = append(required, "actions")
	}
	return object(properties, required...)
}
