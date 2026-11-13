package service

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type templateVariable struct {
	Key          string   `json:"key"`
	Type         string   `json:"type"`
	Value        any      `json:"value"`
	Options      []string `json:"options"`
	Required     *bool    `json:"required"`
	ChoiceOption []string `json:"choice_option"`
	Credential   bool     `json:"is_credential"`
}

type missingTemplateVariables struct{ Keys []string }

func (e *missingTemplateVariables) Error() string {
	return "请补齐模板必填变量：" + strings.Join(e.Keys, "、")
}

func templateVariables(raw []byte, values map[string]string) ([]map[string]any, map[string]any, error) {
	if err := validateVariables(values); err != nil {
		return nil, nil, err
	}
	var definition struct {
		Variables        []templateVariable `json:"variables"`
		ServiceVariables []templateVariable `json:"service_variable_kvs"`
		PreBuild         struct {
			Envs []templateVariable `json:"envs"`
		} `json:"pre_build"`
	}
	var variables []templateVariable
	serviceVariables := false
	if len(raw) > 0 && raw[0] == '[' {
		serviceVariables = true
		if err := json.Unmarshal(raw, &variables); err != nil {
			return nil, nil, err
		}
	} else {
		if json.Unmarshal(raw, &definition) != nil {
			return nil, nil, fmt.Errorf("模板变量定义无效")
		}

		variables = definition.ServiceVariables
		serviceVariables = len(variables) > 0
		if len(variables) == 0 {
			variables = definition.Variables
		}

		if len(variables) == 0 {
			variables = definition.PreBuild.Envs
		}
	}

	known := map[string]bool{}
	overrides := map[string]any{}
	list := []map[string]any{}
	missing := []string{}
	for _, variable := range variables {
		if variable.Type == "choice" && len(variable.Options) == 0 {
			variable.Options = variable.ChoiceOption
		}
		known[variable.Key] = true
		value, provided := values[variable.Key]
		required := variable.Required != nil && *variable.Required
		missingValue := !provided && (variable.Value == nil || variable.Value == "")
		if (required && (missingValue || (provided && strings.TrimSpace(value) == ""))) || (serviceVariables && variable.Required == nil && missingValue) {
			if variable.Type == "yaml" {
				return nil, nil, fmt.Errorf("YAML 类型变量 %s 必须在模板中预先配置", variable.Key)
			}
			missing = append(missing, variable.Key)
			continue
		}
		if !provided {
			continue
		}

		var converted any = value
		switch variable.Type {
		case "bool":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return nil, nil, fmt.Errorf("变量 %s 必须是布尔值", variable.Key)
			}
			converted = parsed
		case "enum", "choice":
			found := false
			for _, option := range variable.Options {
				if option == value {
					found = true
				}
			}
			if !found {
				return nil, nil, fmt.Errorf("变量 %s 不在模板可选值中", variable.Key)
			}
		case "", "string":
		default:
			return nil, nil, fmt.Errorf("不支持通过聊天填写 YAML 类型变量 %s，请使用已有配置", variable.Key)
		}

		kind := variable.Type
		if kind == "" {
			kind = "string"
		}

		list = append(
			list,
			map[string]any{
				"key":           variable.Key,
				"value":         converted,
				"type":          kind,
				"is_credential": variable.Credential,
				"choice_option": variable.Options,
				"options":       variable.Options,
			},
		)
		overrides[variable.Key] = converted
	}

	for key := range values {
		if !known[key] {
			return nil, nil, fmt.Errorf("变量 %s 不在所选模板定义中", key)
		}
	}
	if len(missing) > 0 {
		return list, overrides, &missingTemplateVariables{Keys: missing}
	}

	return list, overrides, nil
}
