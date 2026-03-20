package service

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	templatemodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/util"
)

func cloneWorkLoads(workLoads []*commonmodels.WorkLoad) []*commonmodels.WorkLoad {
	ret := make([]*commonmodels.WorkLoad, 0, len(workLoads))
	for _, item := range workLoads {
		if item == nil {
			continue
		}
		copied := *item
		ret = append(ret, &copied)
	}
	return ret
}

func cloneRenderVariableKVs(kvs []*commontypes.RenderVariableKV) []*commontypes.RenderVariableKV {
	ret := make([]*commontypes.RenderVariableKV, 0, len(kvs))
	for _, kv := range kvs {
		if kv == nil {
			continue
		}
		copied := *kv
		copied.Options = append([]string{}, kv.Options...)
		ret = append(ret, &copied)
	}
	return ret
}

func cloneProductService(service *commonmodels.ProductService) *commonmodels.ProductService {
	if service == nil {
		return nil
	}
	copied := *service
	copied.Containers = append([]*commonmodels.Container{}, service.Containers...)
	copied.Resources = append([]*commonmodels.ServiceResource{}, service.Resources...)
	copied.WorkLoads = cloneWorkLoads(service.WorkLoads)
	copied.Render = cloneServiceRender(service.GetServiceRender())
	return &copied
}

func cloneServiceRender(render *templatemodels.ServiceRender) *templatemodels.ServiceRender {
	if render == nil {
		return nil
	}
	copied := *render
	if render.OverrideYaml != nil {
		overrideCopied := *render.OverrideYaml
		overrideCopied.RenderVariableKVs = cloneRenderVariableKVs(render.OverrideYaml.RenderVariableKVs)
		copied.OverrideYaml = &overrideCopied
	}
	return &copied
}

func loadServiceTemplateByRevision(service *commonmodels.ProductService, production bool) (*commonmodels.Service, error) {
	if service == nil {
		return nil, fmt.Errorf("service is nil")
	}
	return repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ServiceName: service.ServiceName,
		ProductName: service.ProductName,
		Type:        service.Type,
		Revision:    service.Revision,
	}, production)
}

func mergeServiceRenderVariableKVs(templateVars []*commontypes.ServiceVariableKV, kvGroups ...[]*commontypes.RenderVariableKV) ([]*commontypes.RenderVariableKV, error) {
	mergedInput := [][]*commontypes.RenderVariableKV{commontypes.ServiceToRenderVariableKVs(templateVars)}
	for _, group := range kvGroups {
		mergedInput = append(mergedInput, cloneRenderVariableKVs(group))
	}
	_, ret, err := commontypes.MergeRenderVariableKVs(mergedInput...)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func renderServiceWithOverrides(prod *commonmodels.Product, service *commonmodels.ProductService, tmpl *commonmodels.Service, overrides []*commonmodels.WorkLoad) (string, error) {
	serviceCopy := cloneProductService(service)
	if serviceCopy == nil {
		return "", fmt.Errorf("service is nil")
	}
	serviceCopy.WorkLoads = cloneWorkLoads(overrides)
	return kube.RenderEnvServiceWithTempl(prod, serviceCopy.GetServiceRender(), serviceCopy, tmpl)
}

func serviceReplicaStateChanged(currentSvc, candidateSvc *commonmodels.ProductService) bool {
	if currentSvc == nil && candidateSvc == nil {
		return false
	}
	if currentSvc == nil || candidateSvc == nil {
		return true
	}
	currentRender := currentSvc.GetServiceRender().OverrideYaml
	candidateRender := candidateSvc.GetServiceRender().OverrideYaml
	return !reflect.DeepEqual(currentRender.RenderVariableKVs, candidateRender.RenderVariableKVs) ||
		currentRender.YamlContent != candidateRender.YamlContent ||
		!reflect.DeepEqual(currentSvc.WorkLoads, candidateSvc.WorkLoads)
}

// syncServiceReplicaOverrides 通过对比当前与候选服务渲染结果，重新计算需要保留/更新的副本 override。
// preloadedServiceTemplate 用于复用调用方已查询到的服务模板，命中 revision 时可避免重复查询。
func syncServiceReplicaOverrides(prod *commonmodels.Product, currentSvc, candidateSvc *commonmodels.ProductService, preloadedServiceTemplate *commonmodels.Service) ([]*commonmodels.WorkLoad, error) {
	if candidateSvc == nil {
		return nil, fmt.Errorf("candidate service is nil")
	}
	if currentSvc == nil {
		return cloneWorkLoads(candidateSvc.WorkLoads), nil
	}

	currentSvc = cloneProductService(currentSvc)
	candidateSvc = cloneProductService(candidateSvc)

	resolveTemplate := func(svc *commonmodels.ProductService, role string) (*commonmodels.Service, error) {
		if preloadedServiceTemplate != nil && preloadedServiceTemplate.Revision == svc.Revision {
			return preloadedServiceTemplate, nil
		}
		tmpl, err := loadServiceTemplateByRevision(svc, prod.Production)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s service template %s: %w", role, svc.ServiceName, err)
		}
		return tmpl, nil
	}

	currentTmpl, err := resolveTemplate(currentSvc, "current")
	if err != nil {
		return nil, err
	}
	candidateTmpl := currentTmpl
	if candidateSvc.Revision != currentSvc.Revision {
		candidateTmpl, err = resolveTemplate(candidateSvc, "candidate")
		if err != nil {
			return nil, err
		}
	}

	return reconcileReplicaOverrides(prod, currentSvc, currentTmpl, candidateSvc, candidateTmpl)
}

// syncUpdatedProductReplicaOverrides 对更新后的产品中所有 k8s 服务执行副本 override 对齐（含升版服务）。
func syncUpdatedProductReplicaOverrides(prod *commonmodels.Product, currentSvcSnapshotMap map[string]*commonmodels.ProductService, serviceRevisionMap map[string]*SvcRevision, updateRevisionSvcs []string, latestServiceMap map[string]*commonmodels.Service) error {
	for _, prodServiceGroup := range prod.Services {
		for _, prodService := range prodServiceGroup {
			if prodService == nil || prodService.Type != setting.K8SDeployType {
				continue
			}

			serviceKey := serviceNameTypeKey(prodService.ServiceName, prodService.Type)
			currentSvcSnapshot := currentSvcSnapshotMap[serviceKey]
			if currentSvcSnapshot == nil {
				continue
			}

			candidateSvc := cloneProductService(prodService)
			if util.InStringArray(candidateSvc.ServiceName, updateRevisionSvcs) {
				if svcRev, ok := serviceRevisionMap[serviceKey]; ok {
					if svcRev.NextRevision > 0 {
						candidateSvc.Revision = svcRev.NextRevision
					}
					candidateSvc.Containers = svcRev.Containers
				}
			}

			overrides, err := syncServiceReplicaOverrides(prod, currentSvcSnapshot, candidateSvc, latestServiceMap[serviceKey])
			if err != nil {
				return fmt.Errorf("failed to reconcile replica overrides for service %s: %w", prodService.ServiceName, err)
			}
			prodService.WorkLoads = overrides
		}
	}
	return nil
}

// reconcileReplicaOverrides 始终以候选服务（模板+变量渲染）的 replicas 为准，生成完整副本 override。
func reconcileReplicaOverrides(prod *commonmodels.Product, currentSvc *commonmodels.ProductService, currentTmpl *commonmodels.Service, candidateSvc *commonmodels.ProductService, candidateTmpl *commonmodels.Service) ([]*commonmodels.WorkLoad, error) {
	_ = currentSvc
	_ = currentTmpl

	candidateYaml, err := renderServiceWithOverrides(prod, candidateSvc, candidateTmpl, nil)
	if err != nil {
		return nil, err
	}
	candidateReplicaMap, err := kube.ExtractWorkloadReplicas(candidateYaml)
	if err != nil {
		return nil, err
	}

	baseOverrides := make([]*commonmodels.WorkLoad, 0, len(candidateReplicaMap))

	keys := make([]string, 0, len(candidateReplicaMap))
	for key := range candidateReplicaMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		candidateReplica := candidateReplicaMap[key]
		workloadType, workloadName := "", key
		if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
			workloadType = kube.NormalizeReplicaWorkloadType(parts[0])
			workloadName = parts[1]
		}
		baseOverrides, err = kube.UpsertWorkLoadsReplicas(baseOverrides, workloadType, workloadName, candidateReplica)
		if err != nil {
			return nil, err
		}
	}

	return baseOverrides, nil
}
