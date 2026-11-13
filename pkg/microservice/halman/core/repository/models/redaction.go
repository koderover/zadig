package models

import "encoding/json"

// 展示副本隐藏凭据；执行及草稿恢复始终使用数据库里的原参数。
func (p OperationPlan) Redacted() OperationPlan {
	raw, _ := json.Marshal(p)
	var result OperationPlan
	_ = json.Unmarshal(raw, &result)
	result.DraftChecks = nil
	if result.Draft != nil {
		for i := range result.Draft.Services {
			item := &result.Draft.Services[i]
			maskVariables(item.Variables, item.CredentialVariables)
		}
		for i := range result.Draft.Builds {
			item := &result.Draft.Builds[i]
			maskVariables(item.Variables, item.CredentialVariables)
		}
	}
	for i := range result.Items {
		result.Items[i] = result.Items[i].Redacted()
	}
	return result
}

func (item OperationItem) Redacted() OperationItem {
	switch item.Action {
	case "create_service":
		var input ServiceInput
		if json.Unmarshal(item.Params, &input) == nil {
			maskVariables(input.Variables, input.CredentialVariables)
			item.Params, _ = json.Marshal(input)
		}
	case "create_build":
		var input BuildInput
		if json.Unmarshal(item.Params, &input) == nil {
			maskVariables(input.Variables, input.CredentialVariables)
			item.Params, _ = json.Marshal(input)
		}
	}
	return item
}

func maskVariables(values map[string]string, keys []string) {
	for _, key := range keys {
		if _, exists := values[key]; exists {
			values[key] = "[已隐藏]"
		}
	}
}

func (r Run) MarshalJSON() ([]byte, error) {
	type plain Run
	var plan OperationPlan
	if len(r.Plan) > 0 && json.Unmarshal(r.Plan, &plan) == nil {
		r.Plan, _ = json.Marshal(plan.Redacted())
	}
	return json.Marshal(plain(r))
}
