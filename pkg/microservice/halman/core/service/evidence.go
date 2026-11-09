package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

var workflowJobName = regexp.MustCompile(`^[a-zA-Z0-9\p{Han}][a-zA-Z0-9\p{Han}_.-]{0,127}$`)

// 步骤名可能包含中文，不能沿用项目或 Kubernetes 资源的纯英文标识校验。
func checkWorkflowJobName(name string) error {
	if name == "" {
		return fmt.Errorf("需要明确工作流步骤名称")
	}
	if !workflowJobName.MatchString(name) {
		return fmt.Errorf("工作流步骤名称不合法")
	}
	return nil
}

func (s *AgentService) workflowLog(ctx context.Context, args models.WorkflowLogWindow) (any, error) {
	e, err := s.skillExecution(ctx, "workflow_diagnosis")
	if err != nil {
		return nil, err
	}

	if args.Offset < 0 || args.Offset > 5000 || args.Limit < 1 || args.Limit > 500 {
		return nil, fmt.Errorf("日志窗口超出允许范围")
	}

	if err := checkNames(args.Target.ProjectKey, args.Target.WorkflowName); err != nil {
		return nil, err
	}
	if err := checkWorkflowJobName(args.Target.JobName); err != nil {
		return nil, err
	}
	if args.Target.TaskID < 1 {
		return nil, fmt.Errorf("需要明确工作流任务编号")
	}

	// 原日志片段接口校验查看权限及任务、步骤归属，不再预读完整任务。
	raw, err := s.Platform.business(
		ctx,
		e.Authorization,
		http.MethodGet,
		fmt.Sprintf(
			"/api/logs/log/v4/workflow/%s/tasks/%d/jobs/%s",
			args.Target.WorkflowName,
			args.Target.TaskID,
			url.PathEscape(args.Target.JobName),
		),
		url.Values{
			"projectName": {args.Target.ProjectKey},
			"format":      {"excerpt"},
			"offset":      {strconv.Itoa(args.Offset)},
			"limit":       {strconv.Itoa(args.Limit)},
		},
		nil,
	)
	if err != nil {
		return nil, err
	}

	if err := s.recordScope(
		ctx,
		e,
		models.PageContext{
			ProjectKey:   args.Target.ProjectKey,
			WorkflowName: args.Target.WorkflowName,
			TaskID:       args.Target.TaskID,
		},
	); err != nil {
		return nil, err
	}

	e.EvidenceRead.Store(true)
	return safeJSON(raw)
}

type servicePod struct {
	Name            string `json:"pod_name"`
	Ready           bool   `json:"pod_ready"`
	ContainersReady bool   `json:"containers_ready"`
	Created         int64  `json:"create_time"`
	Containers      []struct {
		Name string `json:"name"`
	} `json:"containers"`
}

type servicePods struct {
	ServiceName string       `json:"service_name"`
	Truncated   bool         `json:"truncated"`
	Pods        []servicePod `json:"pods"`
}

type environmentVersion struct {
	Revision   int64  `json:"revision"`
	Detail     string `json:"detail"`
	Created    int64  `json:"create_time"`
	Containers []struct {
		Name  string `json:"name"`
		Image string `json:"image"`
	} `json:"containers"`
}

func (p *PlatformClient) versions(ctx context.Context, auth string, args models.RepairInput) ([]environmentVersion, error) {
	raw, err := p.business(
		ctx,
		auth,
		http.MethodGet,
		"/api/environment/environments/"+args.Target.EnvName+"/version/"+args.Target.ServiceName,
		repairQuery(args),
		nil,
	)
	if err != nil {
		return nil, err
	}

	var result []environmentVersion
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}

	for i := range result {
		result[i].Detail = redactText(result[i].Detail)
	}

	return result, nil
}

func serviceQuery(t models.ServiceTarget) url.Values {
	return url.Values{
		"projectName": {t.ProjectKey},
		"production":  {strconv.FormatBool(t.Production)},
		"isHelmChart": {"false"},
	}
}

func (p *PlatformClient) servicePods(ctx context.Context, auth string, t models.ServiceTarget) (*servicePods, error) {
	if err := checkNames(t.ProjectKey, t.EnvName, t.ServiceName); err != nil {
		return nil, err
	}
	path := "/openapi/environments/"
	if t.Production {
		path += "production/"
	}
	path += t.EnvName + "/service/" + t.ServiceName + "/pods"
	raw, err := p.business(ctx, auth, http.MethodGet, path, url.Values{"projectKey": {t.ProjectKey}}, nil)
	if err != nil {
		return nil, err
	}
	var result servicePods
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	if result.ServiceName != t.ServiceName {
		return nil, fmt.Errorf("服务实例归属与目标不匹配")
	}
	result.Truncated = len(result.Pods) > 50
	if result.Truncated {
		result.Pods = result.Pods[:50]
	}
	return &result, nil
}

func (s *AgentService) serviceLogs(ctx context.Context, e *execution, t models.ServiceTarget) any {
	pods, err := s.Platform.servicePods(ctx, e.Authorization, t)
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	logs, events := map[string]any{}, map[string]any{}
	budget := 64 << 10
	save := func(target map[string]any, key string, value any) {
		size := len(common.JSON(value))
		if size > budget {
			target[key] = map[string]string{"unavailable": "本轮日志和事件证据达到大小上限"}
			return
		}
		budget -= size
		target[key] = value
	}
	count := 0
	for _, pod := range pods.Pods {
		for _, container := range pod.Containers {
			if count >= 3 {
				break
			}
			if checkNames(pod.Name, container.Name) != nil {
				continue
			}
			count++
			if _, known := events[pod.Name]; !known {
				query := serviceQuery(t)
				query.Set("envName", t.EnvName)
				raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/environment/kube/pods/"+pod.Name+"/events", query, nil)
				var items []json.RawMessage
				if err == nil {
					err = json.Unmarshal(raw, &items)
				}
				if err != nil {
					events[pod.Name] = map[string]string{"unavailable": "Pod 事件不可用"}
				} else {
					truncated := len(items) > 20
					if truncated {
						items = items[:20]
					}
					value, safeErr := safeJSON(common.JSON(items))
					if safeErr != nil {
						events[pod.Name] = map[string]string{"unavailable": safeErr.Error()}
					} else {
						save(events, pod.Name, map[string]any{"items": value, "truncated": truncated})
					}
				}
			}
			for _, previous := range []bool{false, true} {
				key := pod.Name + "/" + container.Name + "/previous=" + strconv.FormatBool(previous)
				if budget < 1024 {
					logs[key] = map[string]string{"unavailable": "本轮日志证据达到大小上限"}
					continue
				}
				query := url.Values{
					"projectName": {t.ProjectKey}, "envName": {t.EnvName}, "serviceName": {t.ServiceName},
					"container": {container.Name}, "previous": {strconv.FormatBool(previous)},
					"format": {"excerpt"}, "follow": {"false"}, "tailLines": {"200"},
				}
				raw, err := s.Platform.business(ctx, e.Authorization, http.MethodGet, "/api/logs/pods/"+pod.Name, query, nil)
				if err != nil {
					logs[key] = map[string]string{"unavailable": err.Error()}
					continue
				}
				value, err := safeJSON(raw)
				if err != nil {
					logs[key] = map[string]string{"unavailable": err.Error()}
					continue
				}
				save(logs, key, value)
			}
		}
	}
	return map[string]any{
		"pods": pods, "container_evidence": logs, "pod_events": events,
		"note": "本次最多选择三个容器，按共享大小预算读取当前和上次退出日志；Pod 事件通过已有接口单独查询。不可用的日志不能作为已取得的证据。",
	}
}
