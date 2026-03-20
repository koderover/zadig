package service

import (
	"context"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/openkruise/kruise-api/apps/v1alpha1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/kube"
	commontypes "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/types"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/kube/updater"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

// Scale 先校验副本来源，再执行集群副本变更，最后在需要时持久化环境状态与版本记录。
func Scale(args *ScaleArgs, updateBy string, logger *zap.SugaredLogger) error {
	opt := &commonrepo.ProductFindOptions{
		Name:       args.ProductName,
		EnvName:    args.EnvName,
		Production: &args.Production,
	}
	prod, err := commonrepo.NewProductColl().Find(opt)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}
	if prod.IsSleeping() {
		return e.ErrScaleService.AddErr(fmt.Errorf("environment is sleeping"))
	}

	project, err := templaterepo.NewProductColl().Find(args.ProductName)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	kubeClient, err := clientmanager.NewKubeClientManager().GetControllerRuntimeClient(prod.ClusterID)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	if !project.IsK8sYamlProduct() {
		return scaleWorkload(prod.Namespace, args.Type, args.Name, args.Number, kubeClient, logger)
	}

	mutexAutoUpdate := cache.NewRedisLock(updateMultipleProductLockKey(args.ProductName))
	if err := mutexAutoUpdate.Lock(); err != nil {
		return e.ErrScaleService.AddErr(fmt.Errorf("failed to acquire lock, err: %s", err))
	}
	defer func() {
		mutexAutoUpdate.Unlock()
	}()

	currentSvc, currentTmpl, err := findScaleTargetService(prod, args.ServiceName, args.Type, args.Name)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	source, err := resolveScaleReplicaSource(prod, currentSvc, currentTmpl, args.Type, args.Name)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}
	if source.Kind == replicaSourceGlobal {
		return e.ErrScaleService.AddErr(fmt.Errorf("replicas of workload %s/%s is sourced from environment global variables and cannot be updated by scale", args.Type, args.Name))
	}

	liveReplica, err := getWorkloadLiveReplica(prod.Namespace, args.Type, args.Name, kubeClient)
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	candidateSvc := cloneProductService(currentSvc)
	switch source.Kind {
	case replicaSourceLiteral:
	case replicaSourceService:
		mergedRenderKVs, err := mergeServiceRenderVariableKVs(currentTmpl.ServiceVariableKVs, currentSvc.GetServiceRender().OverrideYaml.RenderVariableKVs)
		if err != nil {
			return e.ErrScaleService.AddErr(err)
		}
		updatedRenderKVs, err := updateRenderVariableReplicaValue(mergedRenderKVs, source.RootKey, source.SubPath, args.Number)
		if err != nil {
			return e.ErrScaleService.AddErr(err)
		}
		candidateSvc.GetServiceRender().OverrideYaml.RenderVariableKVs = updatedRenderKVs
		candidateSvc.GetServiceRender().OverrideYaml.YamlContent, err = commontypes.RenderVariableKVToYaml(updatedRenderKVs, true)
		if err != nil {
			return e.ErrScaleService.AddErr(err)
		}
	default:
		return e.ErrScaleService.AddErr(fmt.Errorf("unsupported replicas source for workload %s/%s", args.Type, args.Name))
	}
	candidateSvc.WorkLoads, err = kube.UpsertWorkLoadsReplicas(candidateSvc.WorkLoads, args.Type, args.Name, int32(args.Number))
	if err != nil {
		return e.ErrScaleService.AddErr(err)
	}
	candidateSvc.UpdateTime = time.Now().Unix()

	envStateChanged := serviceReplicaStateChanged(currentSvc, candidateSvc)
	targetReplica := int32(args.Number)
	if liveReplica == targetReplica && !envStateChanged {
		return nil
	}

	if liveReplica != targetReplica {
		if err := scaleWorkload(prod.Namespace, args.Type, args.Name, args.Number, kubeClient, logger); err != nil {
			return e.ErrScaleService.AddErr(err)
		}
	}

	if !envStateChanged {
		return nil
	}

	if err := updateEnvService(prod, candidateSvc); err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	session := mongotool.Session()
	defer session.EndSession(context.Background())
	if err := mongotool.StartTransaction(session); err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	productColl := commonrepo.NewProductCollWithSession(session)
	if err := productColl.Update(prod); err != nil {
		mongotool.AbortTransaction(session)
		return e.ErrScaleService.AddErr(err)
	}

	if err := commonutil.CreateEnvServiceVersion(prod, candidateSvc, updateBy, config.EnvOperationDefault, "", session, logger); err != nil {
		mongotool.AbortTransaction(session)
		return e.ErrScaleService.AddErr(err)
	}

	if err := mongotool.CommitTransaction(session); err != nil {
		return e.ErrScaleService.AddErr(err)
	}

	return nil
}

func OpenAPIScale(req *OpenAPIScaleServiceReq, updateBy string, logger *zap.SugaredLogger) error {
	args := &ScaleArgs{
		Type:        req.WorkloadType,
		ProductName: req.ProjectKey,
		EnvName:     req.EnvName,
		ServiceName: req.ServiceName,
		Name:        req.WorkloadName,
		Number:      req.TargetReplicas,
		Production:  false,
	}

	return Scale(args, updateBy, logger)
}

func scaleWorkload(namespace, workloadType, workloadName string, replicas int, kubeClient client.Client, logger *zap.SugaredLogger) error {
	switch kube.NormalizeReplicaWorkloadType(workloadType) {
	case setting.Deployment:
		if err := updater.ScaleDeployment(namespace, workloadName, replicas, kubeClient); err != nil {
			logger.Errorf("failed to scale %s/deployment/%s to %d", namespace, workloadName, replicas)
			return err
		}
	case setting.StatefulSet:
		if err := updater.ScaleStatefulSet(namespace, workloadName, replicas, kubeClient); err != nil {
			logger.Errorf("failed to scale %s/statefulset/%s to %d", namespace, workloadName, replicas)
			return err
		}
	case setting.CloneSet:
		if err := updater.ScaleCloneSet(namespace, workloadName, replicas, kubeClient); err != nil {
			logger.Errorf("failed to scale %s/cloneset/%s to %d", namespace, workloadName, replicas)
			return err
		}
	default:
		return fmt.Errorf("unsupported workload type: %s", workloadType)
	}
	return nil
}

func getWorkloadLiveReplica(namespace, workloadType, workloadName string, kubeClient client.Client) (int32, error) {
	switch kube.NormalizeReplicaWorkloadType(workloadType) {
	case setting.Deployment:
		obj := &appsv1.Deployment{}
		if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: workloadName}, obj); err != nil {
			return 0, err
		}
		if obj.Spec.Replicas == nil {
			return 1, nil
		}
		return *obj.Spec.Replicas, nil
	case setting.StatefulSet:
		obj := &appsv1.StatefulSet{}
		if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: workloadName}, obj); err != nil {
			return 0, err
		}
		if obj.Spec.Replicas == nil {
			return 1, nil
		}
		return *obj.Spec.Replicas, nil
	case setting.CloneSet:
		obj := &v1alpha1.CloneSet{}
		if err := kubeClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: workloadName}, obj); err != nil {
			return 0, err
		}
		if obj.Spec.Replicas == nil {
			return 1, nil
		}
		return *obj.Spec.Replicas, nil
	default:
		return 0, fmt.Errorf("unsupported workload type: %s", workloadType)
	}
}

// findScaleTargetService 根据 serviceName 解析目标服务及模板，并校验目标 workload 确实属于该服务。
func findScaleTargetService(prod *commonmodels.Product, serviceName, workloadType, workloadName string) (*commonmodels.ProductService, *commonmodels.Service, error) {
	service := prod.GetServiceMap()[serviceName]
	if service == nil || service.Type != setting.K8SDeployType {
		return nil, nil, fmt.Errorf("failed to find k8s service %s in env %s", serviceName, prod.EnvName)
	}

	tmpl, err := loadServiceTemplateByRevision(service, prod.Production)
	if err != nil {
		return nil, nil, err
	}

	renderedYaml, err := renderServiceWithOverrides(prod, service, tmpl, service.WorkLoads)
	if err != nil {
		return nil, nil, err
	}
	_, found, err := kube.GetWorkloadReplica(renderedYaml, workloadType, workloadName)
	if err != nil {
		return nil, nil, err
	}
	if !found {
		return nil, nil, fmt.Errorf("workload %s/%s does not belong to service %s", workloadType, workloadName, serviceName)
	}
	return service, tmpl, nil
}

func updateEnvService(prod *commonmodels.Product, service *commonmodels.ProductService) error {
	for _, group := range prod.Services {
		for index, current := range group {
			if current.ServiceName == service.ServiceName && current.Type == service.Type {
				group[index] = service
				return nil
			}
		}
	}
	return fmt.Errorf("failed to replace service %s in env %s", service.ServiceName, prod.EnvName)
}
