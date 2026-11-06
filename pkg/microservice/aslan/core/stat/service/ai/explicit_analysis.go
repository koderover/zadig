package ai

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ValidateExplicitAnalysisArgs 显式指定范围时不再让模型或时间解析服务覆盖范围。
func ValidateExplicitAnalysisArgs(args *AiAnalysisReq) error {
	if args == nil || strings.TrimSpace(args.Prompt) == "" {
		return fmt.Errorf("分析问题不能为空")
	}
	if len(args.ProjectList) == 0 && args.StartTime == 0 && args.EndTime == 0 {
		return nil
	}
	if len(args.ProjectList) == 0 || args.StartTime <= 0 || args.EndTime <= args.StartTime || args.EndTime > time.Now().Unix() {
		return fmt.Errorf("显式分析必须提供项目列表和有效起止时间，不能包含未来时间")
	}
	for _, name := range args.ProjectList {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("项目名称不能为空")
		}
	}
	return nil
}

func resolveAnalysisProjects(requested []string, names map[string]string) ([]string, error) {
	result := []string{}
	seen := map[string]bool{}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		id := name
		if _, found := names[id]; !found {
			matches := []string{}
			for key, alias := range names {
				if alias == name {
					matches = append(matches, key)
				}
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("项目 %s 不存在，请确认项目标识或展示名称", name)
			}
			if len(matches) > 1 {
				sort.Strings(matches)
				return nil, fmt.Errorf("项目名称 %s 存在多个匹配，请选择项目标识：%s", name, strings.Join(matches, "、"))
			}
			id = matches[0]
		}
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	return result, nil
}
