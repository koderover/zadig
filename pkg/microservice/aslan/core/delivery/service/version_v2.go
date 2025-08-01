/*
Copyright 2025 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/blang/semver/v4"
	"github.com/chartmuseum/helm-push/pkg/helm"
	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	chartloader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	workflowservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/workflow/service/workflow"
	"github.com/koderover/zadig/v2/pkg/setting"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	stepspec "github.com/koderover/zadig/v2/pkg/types/step"
	"github.com/koderover/zadig/v2/pkg/util"
	fsutil "github.com/koderover/zadig/v2/pkg/util/fs"
	yamlutil "github.com/koderover/zadig/v2/pkg/util/yaml"
)

const (
	VerbosityBrief                            string = "brief"    // brief delivery data
	VerbosityDetailed                         string = "detailed" // detailed delivery version with total data
	deliveryVersionWorkflowV4NamingConvention string = "zadig-delivery-%s-%s"
)

type CreateDeliveryVersionRequest struct {
	Version               string                                 `json:"version"`
	ProjectName           string                                 `json:"project_name"`
	EnvName               string                                 `json:"env_name"`
	Production            bool                                   `json:"production"`
	Source                setting.DeliveryVersionSource          `json:"source"`
	Labels                []string                               `json:"labels"`
	Desc                  string                                 `json:"desc"`
	ImageRegistryID       string                                 `json:"image_registry_id"`
	OriginalChartRepoName string                                 `json:"original_chart_repo_name"`
	ChartRepoName         string                                 `json:"chart_repo_name"`
	Services              []*commonmodels.DeliveryVersionService `json:"services"`
	CreateBy              string                                 `json:"create_by"`
}

func CreateK8SDeliveryVersionV2(args *CreateDeliveryVersionRequest, logger *zap.SugaredLogger) error {
	// prepare registry data
	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	targetRegistry, err := checkDeliveryVersionRegistry(args, registryMap)
	if err != nil {
		return err
	}

	// create version data
	versionObj := &commonmodels.DeliveryVersionV2{
		Version:         args.Version,
		ProjectName:     args.ProjectName,
		EnvName:         args.EnvName,
		Production:      args.Production,
		Type:            setting.DeliveryVersionTypeYaml,
		Source:          args.Source,
		Desc:            args.Desc,
		Labels:          args.Labels,
		Status:          setting.DeliveryVersionStatusCreating,
		ImageRegistryID: args.ImageRegistryID,
		Services:        args.Services,
		CreatedBy:       args.CreateBy,
		CreatedAt:       time.Now().Unix(),
	}

	err = commonrepo.NewDeliveryVersionV2Coll().Create(versionObj)
	if err != nil {
		logger.Errorf("failed to insert version data, err: %s", err)
		if mongo.IsDuplicateKeyError(err) {
			return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version %s, version already exist", versionObj.Version))
		}
		return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version: %s, %v", versionObj.Version, err))
	}

	err = buildDeliveryImagesV2(targetRegistry, registryMap, versionObj, logger)
	if err != nil {
		return err
	}

	return nil
}

func getImageSourceRegistryV2(imageData *commonmodels.DeliveryVersionImage, registryMap map[string]*commonmodels.RegistryNamespace) (*commonmodels.RegistryNamespace, error) {
	sourceImageTag := ""
	registryURL := strings.TrimSuffix(imageData.SourceImage, fmt.Sprintf("/%s", imageData.ImageName))
	tmpArr := strings.Split(imageData.SourceImage, ":")
	if len(tmpArr) == 2 {
		sourceImageTag = tmpArr[1]
		registryURL = strings.TrimSuffix(imageData.SourceImage, fmt.Sprintf("/%s:%s", imageData.ImageName, sourceImageTag))
	} else if len(tmpArr) == 3 {
		sourceImageTag = tmpArr[2]
		registryURL = strings.TrimSuffix(imageData.SourceImage, fmt.Sprintf("/%s:%s", imageData.ImageName, sourceImageTag))
	} else if len(tmpArr) == 1 {
		// no need to trim
	} else {
		return nil, fmt.Errorf("invalid image: %s", imageData.SourceImage)
	}
	sourceRegistry, ok := registryMap[registryURL]
	if !ok {
		return nil, fmt.Errorf("can't find source registry for image: %s", imageData.SourceImage)
	}
	return sourceRegistry, nil
}

func buildDeliveryImagesV2(targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace, deliveryVersion *commonmodels.DeliveryVersionV2, logger *zap.SugaredLogger) (err error) {
	defer func() {
		if err != nil {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			deliveryVersion.Error = err.Error()
		}
		updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)
	}()

	// update service image in yaml
	for _, service := range deliveryVersion.Services {
		err = updateDeliveryServiceImageInYaml(service, deliveryVersion.Source)
		if err != nil {
			return fmt.Errorf("failed to update service image in yaml, serviceName: %s, err: %s", service.ServiceName, err)
		}
	}

	// create workflow task to deal with images
	deliveryVersionWorkflowV4, err := generateWorkflowV4FromDeliveryVersionV2(deliveryVersion, targetRegistry, registryMap)
	if err != nil {
		return fmt.Errorf("failed to generate workflow from delivery version, versionName: %s, err: %s", deliveryVersion.Version, err)
	}

	if len(deliveryVersionWorkflowV4.Stages) != 0 {
		createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
			Name: "system",
			Type: config.WorkflowTaskTypeDelivery,
		}, deliveryVersionWorkflowV4, logger)
		if err != nil {
			return fmt.Errorf("failed to create delivery version custom workflow task, versionName: %s, err: %s", deliveryVersion.Version, err)
		}

		deliveryVersion.WorkflowName = createResp.WorkflowName
		deliveryVersion.TaskID = createResp.TaskID
	}

	err = commonrepo.NewDeliveryVersionV2Coll().Update(deliveryVersion)
	if err != nil {
		logger.Errorf("failed to update delivery version, projectName: %s, version: %s, err: %s", deliveryVersion.ProjectName, deliveryVersion.Version, err)
	}

	// start a new routine to check task results
	go waitK8SImageVersionDoneV2(deliveryVersion)

	return
}

func updateDeliveryServiceImageInYaml(service *commonmodels.DeliveryVersionService, source setting.DeliveryVersionSource) error {
	if source == setting.DeliveryVersionSourceManual {
		return nil
	}
	if source == setting.DeliveryVersionSourceFromVersion && service.YamlContent == "" {
		return nil
	}

	newYamlContent := ""

	yamls := util.SplitYaml(service.YamlContent)
	for _, yamlContent := range yamls {
		var manifest map[string]interface{}
		if err := yaml.Unmarshal([]byte(yamlContent), &manifest); err != nil {
			return fmt.Errorf("解析yaml失败: %w", err)
		}

		kind, ok := manifest["kind"].(string)
		if !ok {
			return fmt.Errorf("yaml缺少kind字段")
		}

		// 只处理workload类型
		workloadKinds := map[string]bool{
			"Deployment":  true,
			"StatefulSet": true,
			"DaemonSet":   true,
			"Job":         true,
			"CronJob":     true,
			"ReplicaSet":  true,
		}
		if !workloadKinds[kind] {
			continue
		}

		// 处理 CronJob 的特殊结构
		if kind == "CronJob" {
			spec, ok := manifest["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少spec字段")
			}
			jobTemplate, ok := spec["jobTemplate"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少jobTemplate字段")
			}
			jobSpec, ok := jobTemplate["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少jobTemplate.spec字段")
			}
			template, ok := jobSpec["template"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少jobTemplate.spec.template字段")
			}
			podSpec, ok := template["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少jobTemplate.spec.template.spec字段")
			}
			containers, ok := podSpec["containers"].([]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少containers字段")
			}
			for _, img := range service.Images {
				for _, c := range containers {
					container, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if container["name"] == img.ContainerName {
						container["image"] = img.TargetImage
					}
				}
			}
		} else {
			// 其它workload结构
			spec, ok := manifest["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少spec字段")
			}
			template, ok := spec["template"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少template字段")
			}
			podSpec, ok := template["spec"].(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少template.spec字段")
			}
			containers, ok := podSpec["containers"].([]interface{})
			if !ok {
				return fmt.Errorf("yaml缺少containers字段")
			}
			for _, img := range service.Images {
				for _, c := range containers {
					container, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if container["name"] == img.ContainerName {
						container["image"] = img.TargetImage
					}
				}
			}
		}

		newYaml, err := yaml.Marshal(manifest)
		if err != nil {
			return fmt.Errorf("序列化yaml失败: %w", err)
		}
		newYamlContent += string(newYaml) + "\n"
	}

	service.YamlContent = newYamlContent
	return nil
}

func generateWorkflowV4FromDeliveryVersionV2(deliveryVersion *commonmodels.DeliveryVersionV2, targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace) (*commonmodels.WorkflowV4, error) {
	name := generateDeliveryWorkflowName(deliveryVersion.ProjectName, deliveryVersion.Version)
	resp := &commonmodels.WorkflowV4{
		Name:             name,
		DisplayName:      name,
		Stages:           nil,
		Project:          deliveryVersion.ProjectName,
		CreatedBy:        "system",
		ConcurrencyLimit: 1,
	}

	stage := make([]*commonmodels.WorkflowStage, 0)
	jobs := make([]*commonmodels.Job, 0)

	registryDatasMap := map[*commonmodels.RegistryNamespace]map[string][]*commonmodels.DeliveryVersionImage{}
	for _, service := range deliveryVersion.Services {
		for _, image := range service.Images {
			if !image.PushImage {
				continue
			}
			sourceRegistry, err := getImageSourceRegistryV2(image, registryMap)
			if err != nil {
				return nil, fmt.Errorf("failed to check registry, err: %v", err)
			}

			if registryDatasMap[sourceRegistry] == nil {
				registryDatasMap[sourceRegistry] = map[string][]*commonmodels.DeliveryVersionImage{}
			}
			if registryDatasMap[sourceRegistry][service.ServiceName] == nil {
				registryDatasMap[sourceRegistry][service.ServiceName] = []*commonmodels.DeliveryVersionImage{}
			}
			registryDatasMap[sourceRegistry][service.ServiceName] = append(registryDatasMap[sourceRegistry][service.ServiceName], image)
		}
	}

	i := 0
	for sourceRegistry, serviceNameImageDatasMap := range registryDatasMap {
		for serviceName, images := range serviceNameImageDatasMap {
			for _, image := range images {
				if !image.PushImage {
					continue
				}

				if targetRegistry == nil {
					return nil, fmt.Errorf("target registry not appointed")
				}

				targets := []*commonmodels.DistributeTarget{}
				target := &commonmodels.DistributeTarget{
					ServiceName:   serviceName,
					ServiceModule: image.ContainerName,
					ImageName:     image.ImageName,
					SourceTag:     image.SourceImageTag,
					TargetTag:     image.TargetImageTag,
				}
				targets = append(targets, target)

				jobs = append(jobs, &commonmodels.Job{
					Name:    fmt.Sprintf("distribute-image-%d", i),
					JobType: config.JobZadigDistributeImage,
					Skipped: false,
					Spec: &commonmodels.ZadigDistributeImageJobSpec{
						Source:           config.SourceRuntime,
						JobName:          fmt.Sprintf("distribute-image-%d", i),
						SourceRegistryID: sourceRegistry.ID.Hex(),
						TargetRegistryID: targetRegistry.ID.Hex(),
						Targets:          targets,
					},
					RunPolicy:      "",
					ServiceModules: nil,
				})
				i++
			}
		}
	}

	if len(jobs) > 0 {
		stage = append(stage, &commonmodels.WorkflowStage{
			Name:     "distribute-image",
			Parallel: false,
			Jobs:     jobs,
		})
		resp.Stages = stage
	}

	return resp, nil
}

func waitK8SImageVersionDoneV2(deliveryVersion *commonmodels.DeliveryVersionV2) {
	waitTimeout := time.After(60 * time.Minute * 1)
	for {
		select {
		case <-waitTimeout:
			updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, setting.DeliveryVersionStatusFailed, "timeout")
			return
		default:
			done, err := checkK8SImageVersionStatusV2(deliveryVersion)
			if err != nil {
				updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, setting.DeliveryVersionStatusFailed, err.Error())
				return
			}
			if done {
				return
			}
		}
		time.Sleep(time.Second * 5)
	}
}

func updateVersionStatusV2(versionName, projectName string, status setting.DeliveryVersionStatus, errStr string) {
	// send hook info when version build finished
	if status == setting.DeliveryVersionStatusSuccess {
		versionInfo, err := commonrepo.NewDeliveryVersionV2Coll().Get(&commonrepo.DeliveryVersionV2Args{
			ProjectName: projectName,
			Version:     versionName,
		})
		if err != nil {
			log.Errorf("failed to find version: %s of project %s, err: %s", versionName, projectName, err)
			return
		}
		if versionInfo.Status == setting.DeliveryVersionStatusSuccess {
			return
		}

		if versionInfo.Type == setting.DeliveryVersionTypeChart {
			templateProduct, err := templaterepo.NewProductColl().Find(projectName)
			if err != nil {
				log.Errorf("updateVersionStatus failed to find template product: %s, err: %s", projectName, err)
			} else {
				hookConfig := templateProduct.DeliveryVersionHook
				if hookConfig != nil && hookConfig.Enable {
					err = sendDeliveryVersionHookV2(versionInfo, hookConfig.HookHost, hookConfig.Path)
					if err != nil {
						log.Errorf("updateVersionStatus failed to send version delivery hook, projectName: %s, err: %s", projectName, err)
					}
				}
			}
		}
	}

	err := commonrepo.NewDeliveryVersionV2Coll().UpdateStatusByName(versionName, projectName, status, errStr)
	if err != nil {
		log.Errorf("failed to update version status, name: %s, err: %s", versionName, err)
	}
}

func RetryCreateK8SDeliveryVersionV2(deliveryVersion *commonmodels.DeliveryVersionV2, logger *zap.SugaredLogger) error {
	if deliveryVersion.Status != setting.DeliveryVersionStatusFailed {
		return fmt.Errorf("can't reCreate version with status:%s", deliveryVersion.Status)
	}

	if deliveryVersion.TaskID != 0 {
		err := workflowservice.RetryWorkflowTaskV4(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID), logger)
		if err != nil {
			return fmt.Errorf("failed to retry workflow task, workflowName: %s, taskID: %d, err: %s", deliveryVersion.WorkflowName, deliveryVersion.TaskID, err)
		}

		// update status
		deliveryVersion.Status = setting.DeliveryVersionStatusRetrying
		updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)
	} else {
		return fmt.Errorf("no workflow task found for version: %s", deliveryVersion.Version)
	}

	return nil
}
func checkK8SImageVersionStatusV2(deliveryVersion *commonmodels.DeliveryVersionV2) (bool, error) {
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess || deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		return true, nil
	}

	workflowTaskExist := true
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			workflowTaskExist = false
		} else {
			return false, fmt.Errorf("failed to find workflow task, workflowName: %s, taskID: %d", deliveryVersion.WorkflowName, deliveryVersion.TaskID)
		}
	}

	done := false
	if workflowTaskExist {
		if len(workflowTask.Stages) != 1 {
			return false, fmt.Errorf("invalid task data, stage length not leagal")
		}
		if workflowTask.Status == config.StatusPassed {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
			done = true
		} else if workflowTask.Status == config.StatusFailed || workflowTask.Status == config.StatusTimeout || workflowTask.Status == config.StatusCancelled {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			done = true
		}

		changed, err := CheckDeliveryImageStatus(deliveryVersion, workflowTask, log.SugaredLogger())
		if err != nil {
			return false, fmt.Errorf("failed to check delivery image status, err: %v", err)
		}
		if changed {
			err = commonrepo.NewDeliveryVersionV2Coll().UpdateServiceStatus(deliveryVersion)
			if err != nil {
				return false, fmt.Errorf("failed to update delivery version, err: %v", err)
			}
		}
	} else {
		done = true
		deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
	}
	if done {
		updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)
	}

	return done, nil
}

type DeliveryVersionPayloadImage struct {
	ServiceModule string `json:"service_module"`
	Image         string `json:"image"`
}

type DeliveryVersionPayloadChart struct {
	ChartName    string                         `json:"chart_name"`
	ChartVersion string                         `json:"chart_version"`
	ChartUrl     string                         `json:"chart_url"`
	Images       []*DeliveryVersionPayloadImage `json:"images"`
}

type DeliveryVersionHookPayload struct {
	ProjectName string                         `json:"project_name"`
	Version     string                         `json:"version"`
	Status      setting.DeliveryVersionStatus  `json:"status"`
	Error       string                         `json:"error"`
	StartTime   int64                          `json:"start_time"`
	EndTime     int64                          `json:"end_time"`
	Charts      []*DeliveryVersionPayloadChart `json:"charts"`
}

func sendDeliveryVersionHookV2(deliveryVersion *commonmodels.DeliveryVersionV2, host, urlPath string) error {
	projectName, version := deliveryVersion.ProjectName, deliveryVersion.Version
	ret := &DeliveryVersionHookPayload{
		ProjectName: projectName,
		Version:     version,
		Status:      setting.DeliveryVersionStatusSuccess,
		Error:       "",
		StartTime:   deliveryVersion.CreatedAt,
		EndTime:     time.Now().Unix(),
		Charts:      make([]*DeliveryVersionPayloadChart, 0),
	}

	//distributes image + chart
	deliveryDistributes, err := FindDeliveryDistribute(&commonrepo.DeliveryDistributeArgs{
		ReleaseID: deliveryVersion.ID.Hex(),
	}, log.SugaredLogger())
	if err != nil {
		return err
	}

	distributeImageMap := make(map[string][]*DeliveryVersionPayloadImage)
	for _, distributeImage := range deliveryDistributes {
		if distributeImage.DistributeType != config.Image {
			continue
		}
		distributeImageMap[distributeImage.ChartName] = append(distributeImageMap[distributeImage.ChartName], &DeliveryVersionPayloadImage{
			ServiceModule: distributeImage.ServiceName,
			Image:         distributeImage.RegistryName,
		})
	}

	chartRepoName := ""
	for _, distribute := range deliveryDistributes {
		if distribute.DistributeType != config.Chart {
			continue
		}
		ret.Charts = append(ret.Charts, &DeliveryVersionPayloadChart{
			ChartName:    distribute.ChartName,
			ChartVersion: distribute.ChartVersion,
			Images:       distributeImageMap[distribute.ChartName],
		})
		chartRepoName = distribute.ChartRepoName
	}
	err = fillChartUrl(ret.Charts, chartRepoName)
	if err != nil {
		return err
	}

	targetPath := fmt.Sprintf("%s/%s", host, strings.TrimPrefix(urlPath, "/"))

	// validate url
	_, err = url.Parse(targetPath)
	if err != nil {
		return err
	}

	reqBody, err := json.Marshal(ret)
	if err != nil {
		log.Errorf("marshal json args error: %s", err)
		return err
	}

	request, err := http.NewRequest("POST", targetPath, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hook request send to url: %s got error resp code : %d", targetPath, resp.StatusCode)
	}

	return nil
}

func RetryDeliveryVersionV2(id string, logger *zap.SugaredLogger) error {
	deliveryVersion, err := commonrepo.NewDeliveryVersionV2Coll().FindByID(id)
	if err != nil {
		logger.Errorf("failed to query delivery version data, id: %s, error: %s", id, err)
		return fmt.Errorf("failed to query delivery version data, id: %s, error: %s", id, err)
	}

	if deliveryVersion.Type == setting.DeliveryVersionTypeChart {
		return RetryCreateHelmDeliveryVersionV2(deliveryVersion, logger)
	} else if deliveryVersion.Type == setting.DeliveryVersionTypeYaml {
		return RetryCreateK8SDeliveryVersionV2(deliveryVersion, logger)
	}

	return fmt.Errorf("invalid version type: %s", deliveryVersion.Type)
}

func RetryCreateHelmDeliveryVersionV2(deliveryVersion *commonmodels.DeliveryVersionV2, logger *zap.SugaredLogger) error {
	if deliveryVersion.Status != setting.DeliveryVersionStatusFailed {
		return fmt.Errorf("can't reCreate version with status:%s", deliveryVersion.Status)
	}

	registry, err := commonrepo.NewRegistryNamespaceColl().Find(&commonrepo.FindRegOps{
		ID: deliveryVersion.ImageRegistryID,
	})
	if err != nil {
		return fmt.Errorf("failed to find registry, err: %v", err)
	}

	err = buildDeliveryChartsV2(deliveryVersion, registry, logger)
	if err != nil {
		return err
	}

	// update status
	deliveryVersion.Status = setting.DeliveryVersionStatusRetrying
	updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)

	return nil
}

func CreateHelmDeliveryVersionV2(args *CreateDeliveryVersionRequest, logger *zap.SugaredLogger) error {
	// need appoint service info
	if len(args.Services) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("no service info appointed")
	}
	if len(args.ImageRegistryID) == 0 {
		return e.ErrCreateDeliveryVersion.AddDesc("image registry not appointed")
	}

	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	targetRegistry, err := checkDeliveryVersionRegistry(args, registryMap)
	if err != nil {
		return err
	}

	workflowName := generateDeliveryWorkflowName(args.ProjectName, args.Version)
	versionObj := &commonmodels.DeliveryVersionV2{
		Version:         args.Version,
		ProjectName:     args.ProjectName,
		WorkflowName:    workflowName,
		EnvName:         args.EnvName,
		Source:          args.Source,
		Type:            setting.DeliveryVersionTypeChart,
		Desc:            args.Desc,
		Labels:          args.Labels,
		Services:        args.Services,
		ChartRepoName:   args.ChartRepoName,
		ImageRegistryID: args.ImageRegistryID,
		Status:          setting.DeliveryVersionStatusCreating,
		Production:      args.Production,
		CreatedBy:       args.CreateBy,
		CreatedAt:       time.Now().Unix(),
	}

	err = commonrepo.NewDeliveryVersionV2Coll().Create(versionObj)
	if err != nil {
		logger.Errorf("failed to insert version data, err: %s", err)
		if mongo.IsDuplicateKeyError(err) {
			return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version %s, version already exist", versionObj.Version))
		}
		return e.ErrCreateDeliveryVersion.AddErr(fmt.Errorf("failed to insert delivery version: %s, err: %v", versionObj.Version, err))
	}

	err = buildDeliveryChartsV2(versionObj, targetRegistry, logger)
	if err != nil {
		return err
	}

	return nil
}

func checkDeliveryVersionRegistry(args *CreateDeliveryVersionRequest, registryMap map[string]*commonmodels.RegistryNamespace) (*commonmodels.RegistryNamespace, error) {
	var targetRegistry *commonmodels.RegistryNamespace
	if len(args.ImageRegistryID) != 0 {
		for _, registry := range registryMap {
			if registry.ID.Hex() == args.ImageRegistryID {
				targetRegistry = registry
				break
			}
		}
		targetRegistryProjectSet := sets.NewString()
		for _, project := range targetRegistry.Projects {
			targetRegistryProjectSet.Insert(project)
		}
		if !targetRegistryProjectSet.Has(args.ProjectName) && !targetRegistryProjectSet.Has(setting.AllProjects) {
			return nil, fmt.Errorf("registry %s/%s not support project %s", targetRegistry.RegAddr, targetRegistry.Namespace, args.ProjectName)
		}
	}

	for _, service := range args.Services {
		for _, image := range service.Images {
			if !image.PushImage {
				continue
			}
			_, err := getImageSourceRegistryV2(image, registryMap)
			if err != nil {
				return nil, fmt.Errorf("failed to check registry, err: %v", err)
			}
		}
	}
	return targetRegistry, nil
}

func downloadChartV2(projectName, versionName, chartName, chartVersion string, chartRepo *commonmodels.HelmRepo, untar bool) (string, error) {
	chartTGZName := fmt.Sprintf("%s-%s.tgz", chartName, chartVersion)
	chartTGZFileParent, err := makeChartTGZFileDir(projectName, versionName)
	if err != nil {
		return "", err
	}

	chartTGZFilePath := filepath.Join(chartTGZFileParent, chartTGZName)
	if _, err := os.Stat(chartTGZFilePath); err == nil {
		// local cache exists
		return chartTGZFilePath, nil
	}

	hClient, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return "", err
	}

	chartRef := fmt.Sprintf("%s/%s", chartRepo.RepoName, chartName)
	return chartTGZFilePath, hClient.DownloadChart(commonutil.GeneHelmRepo(chartRepo), chartRef, chartVersion, chartTGZFileParent, untar)
}

func buildDeliveryChartsV2(deliveryVersion *commonmodels.DeliveryVersionV2, targetRegistry *commonmodels.RegistryNamespace, logger *zap.SugaredLogger) (err error) {
	defer func() {
		if err != nil {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
			deliveryVersion.Error = err.Error()
		}
		updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)
	}()

	if deliveryVersion.ChartRepoName != "" {
		// push chart to chart repo
		dir, err := makeChartTGZFileDir(deliveryVersion.ProjectName, deliveryVersion.Version)
		if err != nil {
			return err
		}
		repoInfo, err := getChartRepoData(deliveryVersion.ChartRepoName)
		if err != nil {
			log.Errorf("failed to query chart-repo info, productName: %s, repoName: %s, err: %s", deliveryVersion.ProjectName, deliveryVersion.ChartRepoName, err)
			return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s, err: %s", deliveryVersion.ProjectName, deliveryVersion.ChartRepoName, err)
		}

		var originalRepoInfo *commonmodels.HelmRepo
		if deliveryVersion.Source == setting.DeliveryVersionSourceFromVersion && deliveryVersion.OriginalChartRepoName != "" {
			originalRepoInfo, err = getChartRepoData(deliveryVersion.OriginalChartRepoName)
			if err != nil {
				log.Errorf("failed to query chart-repo info, productName: %s, err: %s", deliveryVersion.ProjectName, err)
				return fmt.Errorf("failed to query chart-repo info, productName: %s, repoName: %s", deliveryVersion.ProjectName, deliveryVersion.OriginalChartRepoName)
			}
		}

		var env *commonmodels.Product
		if deliveryVersion.Source == setting.DeliveryVersionSourceFromEnv {
			env, err = commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
				Name:       deliveryVersion.ProjectName,
				EnvName:    deliveryVersion.EnvName,
				Production: &deliveryVersion.Production,
			})
			if err != nil {
				return fmt.Errorf("failed to find project env info, projectName: %s, envName: %s", deliveryVersion.ProjectName, deliveryVersion.EnvName)
			}
		}

		var errLock sync.Mutex
		wg := sync.WaitGroup{}
		errorList := &multierror.Error{}

		appendError := func(err error) {
			errLock.Lock()
			defer errLock.Unlock()
			errorList = multierror.Append(errorList, err)
		}
		for _, service := range deliveryVersion.Services {
			if service.ChartStatus == config.StatusPassed {
				continue
			}

			wg.Add(1)
			go func(service *commonmodels.DeliveryVersionService) {
				defer wg.Done()

				if deliveryVersion.Source == setting.DeliveryVersionSourceFromEnv {
					err := pushDeliveryChartFromEnv(service, env, repoInfo, dir)
					if err != nil {
						logger.Errorf("failed to push chart, serviceName: %s err: %s", service.ServiceName, err)
						appendError(err)
						return
					}
				} else if deliveryVersion.Source == setting.DeliveryVersionSourceFromVersion && service.ChartVersion != "" {
					err := pushDeliveryChartFromVersion(deliveryVersion.ProjectName, deliveryVersion.Version, service, originalRepoInfo, repoInfo, dir)
					if err != nil {
						logger.Errorf("failed to push chart, serviceName: %s err: %s", service.ServiceName, err)
						appendError(err)
						return
					}
				}

				service.ChartStatus = config.StatusPassed
			}(service)
		}
		wg.Wait()

		if errorList.ErrorOrNil() != nil {
			err = errorList.ErrorOrNil()
			return err
		}
	} else {
		for _, service := range deliveryVersion.Services {
			service.ChartStatus = config.StatusPassed
		}
	}

	registryMap, err := buildRegistryMap()
	if err != nil {
		return fmt.Errorf("failed to build registry map")
	}

	// create workflow v4 task to deal with images
	// offline docker images are not supported
	workflowV4, err := generateWorkflowV4FromDeliveryVersionV2(deliveryVersion, targetRegistry, registryMap)
	if err != nil {
		return fmt.Errorf("failed to generate workflow from delivery version, versionName: %s, err: %s", deliveryVersion.Version, err)
	}

	if len(workflowV4.Stages) != 0 {
		createResp, err := workflowservice.CreateWorkflowTaskV4(&workflowservice.CreateWorkflowTaskV4Args{
			Name: "system",
			Type: config.WorkflowTaskTypeDelivery,
		}, workflowV4, logger)
		if err != nil {
			return fmt.Errorf("failed to create helm delivery version custom workflow task, versionName: %s, err: %s", deliveryVersion.Version, err)
		}
		deliveryVersion.TaskID = createResp.TaskID
	}

	deliveryVersion.WorkflowName = workflowV4.Name
	err = commonrepo.NewDeliveryVersionV2Coll().Update(deliveryVersion)
	if err != nil {
		fmtErr := fmt.Errorf("failed to update delivery version, version: %s, projectName: %s, err: %s", deliveryVersion.Version, deliveryVersion.ProjectName, err)
		logger.Error(fmtErr)
		return fmtErr
	}

	// start a new routine to check task results
	go waitHelmChartVersionDoneV2(deliveryVersion)

	return
}

func pushDeliveryChartFromEnv(service *commonmodels.DeliveryVersionService, env *commonmodels.Product, chartRepo *commonmodels.HelmRepo, dir string) error {
	deliveryChartPath, err := ensureChartFilesFromEnv(service, env)
	if err != nil {
		return err
	}

	// write values.yaml file before load
	if err = yaml.Unmarshal([]byte(service.YamlContent), map[string]interface{}{}); err != nil {
		log.Errorf("invalid yaml content, serviceName: %s, yamlContent: %s", service.ServiceName, service.YamlContent)
		return errors.Wrapf(err, "invalid yaml content for service: %s", service.ServiceName)
	}

	updatedValues, err := updateValuesImage(service)
	if err != nil {
		return fmt.Errorf("failed to update values image, serviceName: %s, err: %s", service.ServiceName, err)
	}
	service.YamlContent = string(updatedValues)

	valuesFilePath := filepath.Join(deliveryChartPath, setting.ValuesYaml)
	err = os.WriteFile(valuesFilePath, updatedValues, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write values.yaml file for service %s", service.ServiceName)
	}

	//load chart info from local storage
	chartRequested, err := chartloader.Load(deliveryChartPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load chart info, path %s", deliveryChartPath)
	}

	//set metadata
	chartRequested.Metadata.Name = service.ChartName
	chartRequested.Metadata.Version = service.ChartVersion
	chartRequested.Metadata.AppVersion = service.ChartVersion

	//create local chart package
	chartPackagePath, err := helm.CreateChartPackage(&helm.Chart{Chart: chartRequested}, dir)
	if err != nil {
		return err
	}

	client, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return errors.Wrapf(err, "failed to create chart repo client, repoName: %s", chartRepo.RepoName)
	}

	proxy, err := commonutil.GenHelmChartProxy(chartRepo)
	if err != nil {
		return errors.Wrapf(err, "failed to generate helm chart proxy, repoName: %s", chartRepo.RepoName)
	}

	log.Infof("pushing chart %s to %s...", filepath.Base(chartPackagePath), chartRepo.URL)
	err = client.PushChart(commonutil.GeneHelmRepo(chartRepo), chartPackagePath, proxy)
	if err != nil {
		return errors.Wrapf(err, "failed to push chart: %s", chartPackagePath)
	}
	return nil
}

func pushDeliveryChartFromVersion(projectName, versionName string, service *commonmodels.DeliveryVersionService, originalChartRepoInfo, chartRepo *commonmodels.HelmRepo, dir string) error {
	sourceChartRepoInfo := chartRepo
	if originalChartRepoInfo != nil {
		sourceChartRepoInfo = originalChartRepoInfo
	}

	deliveryChartPath, err := ensureChartFilesFromVersion(projectName, versionName, sourceChartRepoInfo, service)
	if err != nil {
		return fmt.Errorf("failed to ensure chart files from version, err: %v", err)
	}

	// write values.yaml file before load
	if err = yaml.Unmarshal([]byte(service.YamlContent), map[string]interface{}{}); err != nil {
		log.Errorf("invalid yaml content, serviceName: %s, yamlContent: %s", service.ServiceName, service.YamlContent)
		return errors.Wrapf(err, "invalid yaml content for service: %s", service.ServiceName)
	}

	updatedValues, err := updateValuesImage(service)
	if err != nil {
		return fmt.Errorf("failed to update values image, serviceName: %s, err: %s", service.ServiceName, err)
	}
	service.YamlContent = string(updatedValues)

	valuesFilePath := filepath.Join(deliveryChartPath, setting.ValuesYaml)
	err = os.WriteFile(valuesFilePath, updatedValues, 0644)
	if err != nil {
		return errors.Wrapf(err, "failed to write values.yaml file for service %s", service.ServiceName)
	}

	//load chart info from local storage
	chartRequested, err := chartloader.Load(deliveryChartPath)
	if err != nil {
		return errors.Wrapf(err, "failed to load chart info, path %s", deliveryChartPath)
	}

	//set metadata
	chartRequested.Metadata.Name = service.ChartName
	chartRequested.Metadata.Version = service.ChartVersion
	chartRequested.Metadata.AppVersion = service.ChartVersion

	//create local chart package
	chartPackagePath, err := helm.CreateChartPackage(&helm.Chart{Chart: chartRequested}, dir)
	if err != nil {
		return err
	}

	client, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return errors.Wrapf(err, "failed to create chart repo client, repoName: %s", chartRepo.RepoName)
	}

	proxy, err := commonutil.GenHelmChartProxy(chartRepo)
	if err != nil {
		return errors.Wrapf(err, "failed to generate helm chart proxy, repoName: %s", chartRepo.RepoName)
	}

	log.Infof("pushing chart %s to %s...", filepath.Base(chartPackagePath), chartRepo.URL)
	err = client.PushChart(commonutil.GeneHelmRepo(chartRepo), chartPackagePath, proxy)
	if err != nil {
		return errors.Wrapf(err, "failed to push chart: %s", chartPackagePath)
	}
	return nil
}

func ensureChartFilesFromEnv(service *commonmodels.DeliveryVersionService, env *commonmodels.Product) (string, error) {
	envSvc := env.GetServiceMap()[service.ServiceName]
	if envSvc == nil {
		return "", fmt.Errorf("service %s not found in env %s", service.ServiceName, env.EnvName)
	}

	tmplSvc, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
		ProductName: env.ProductName,
		ServiceName: service.ServiceName,
	}, env.Production)
	if err != nil {
		return "", fmt.Errorf("failed to find template service %s", service.ServiceName)
	}

	revisionBasePath := config.LocalDeliveryChartPathWithRevision(env.ProductName, service.ServiceName, envSvc.Revision)
	deliveryChartPath := filepath.Join(revisionBasePath, service.ServiceName)
	if exists, _ := fsutil.DirExists(deliveryChartPath); exists {
		return deliveryChartPath, nil
	}

	serviceName, revision := envSvc.ServiceName, envSvc.Revision
	basePath := config.LocalTestServicePathWithRevision(env.ProductName, serviceName, fmt.Sprint(revision))
	if err := commonutil.PreloadServiceManifestsByRevision(basePath, tmplSvc, env.Production); err != nil {
		log.Warnf("failed to get chart of revision: %d for service: %s, use latest version", revision, serviceName)
		// use the latest version when it fails to download the specific version
		basePath = config.LocalTestServicePath(env.ProductName, serviceName)
		if err = commonutil.PreLoadServiceManifests(basePath, tmplSvc, env.Production); err != nil {
			log.Errorf("failed to load chart info for service %v", service.ServiceName)
			return "", err
		}
	}

	fullPath := filepath.Join(basePath, service.ServiceName)
	err = copy.Copy(fullPath, deliveryChartPath)
	if err != nil {
		return "", err
	}

	helmClient, err := helmtool.NewClientFromNamespace(env.ClusterID, env.Namespace)
	if err != nil {
		log.Errorf("[%s][%s] init helm client error: %s", env.ProductName, env.Namespace, err)
		return "", err
	}

	releaseName := util.GeneReleaseName(tmplSvc.GetReleaseNaming(), env.ProductName, env.Namespace, env.EnvName, service.ServiceName)
	valuesMap, err := helmClient.GetReleaseValues(releaseName, true)
	if err != nil {
		log.Errorf("failed to get values map data, err: %s", err)
		return "", err
	}

	currentValuesYaml, err := yaml.Marshal(valuesMap)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(filepath.Join(deliveryChartPath, setting.ValuesYaml), currentValuesYaml, 0644)
	if err != nil {
		return "", errors.Wrapf(err, "failed to write values.yaml")
	}

	return deliveryChartPath, nil
}

func ensureChartFilesFromVersion(projectName, versionName string, chartRepo *commonmodels.HelmRepo, service *commonmodels.DeliveryVersionService) (string, error) {
	chartNameWithVersion := fmt.Sprintf("%s-%s", service.ChartName, service.OriginalChartVersion)
	chartFilesParent, err := makeChartFilesDir(projectName, versionName)
	if err != nil {
		return "", err
	}

	err = os.RemoveAll(filepath.Join(chartFilesParent, chartNameWithVersion))
	if err != nil {
		return "", errors.Wrapf(err, "failed to remove old chart files")
	}

	hClient, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return "", err
	}

	chartRef := fmt.Sprintf("%s/%s", chartRepo.RepoName, service.ChartName)
	err = hClient.DownloadChart(commonutil.GeneHelmRepo(chartRepo), chartRef, service.OriginalChartVersion, chartFilesParent, true)
	if err != nil {
		return "", fmt.Errorf("failed to download chart %s", chartRef)
	}

	oldPath := filepath.Join(chartFilesParent, service.ChartName)
	newPath := filepath.Join(chartFilesParent, chartNameWithVersion)
	err = os.Rename(oldPath, newPath)
	if err != nil {
		return "", fmt.Errorf("failed rename chart files dir, old: %s, new: %s, err: %s", oldPath, newPath, err)
	}

	log.Debugf("chartRef: %s, deliveryChartPath: %s, chartTGZFileParent: %s", chartRef, newPath, chartFilesParent)

	err = os.WriteFile(filepath.Join(newPath, setting.ValuesYaml), []byte(service.YamlContent), 0644)
	if err != nil {
		return "", errors.Wrapf(err, "failed to write values.yaml")
	}

	return newPath, nil
}

func waitHelmChartVersionDoneV2(deliveryVersion *commonmodels.DeliveryVersionV2) {
	waitTimeout := time.After(60 * time.Minute * 1)
	for {
		select {
		case <-waitTimeout:
			updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, setting.DeliveryVersionStatusFailed, "timeout")
			return
		default:
			done, err := checkHelmChartVersionStatusV2(deliveryVersion)
			if err != nil {
				updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, setting.DeliveryVersionStatusFailed, err.Error())
				return
			}
			if done {
				return
			}
		}
		time.Sleep(time.Second * 5)
	}
}
func checkHelmChartVersionStatusV2(deliveryVersion *commonmodels.DeliveryVersionV2) (bool, error) {
	if deliveryVersion.Status == setting.DeliveryVersionStatusSuccess || deliveryVersion.Status == setting.DeliveryVersionStatusFailed {
		return true, nil
	}

	workflowTaskExist := true
	workflowTask, err := commonrepo.NewworkflowTaskv4Coll().Find(deliveryVersion.WorkflowName, int64(deliveryVersion.TaskID))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			workflowTaskExist = false
		} else {
			return false, fmt.Errorf("failed to find workflow task, workflowName: %s, taskID: %d", deliveryVersion.WorkflowName, deliveryVersion.TaskID)
		}
	}

	taskDone := true
	taskSuccess := true
	if workflowTaskExist {
		if !lo.Contains(config.CompletedStatus(), workflowTask.Status) {
			taskDone = false
		}
		if workflowTask.Status != config.StatusPassed {
			taskSuccess = false
		}

		changed, err := CheckDeliveryImageStatus(deliveryVersion, workflowTask, log.SugaredLogger())
		if err != nil {
			return false, fmt.Errorf("failed to check delivery image status, err: %v", err)
		}
		if changed {
			err = commonrepo.NewDeliveryVersionV2Coll().UpdateServiceStatus(deliveryVersion)
			if err != nil {
				log.Errorf("failed to update delivery version, version: %s, projectName: %s, err: %s", deliveryVersion.Version, deliveryVersion.ProjectName, err)
			}
		}
	}

	if taskDone {
		chartSuccessCount := 0
		for _, service := range deliveryVersion.Services {
			if service.ChartStatus == config.StatusPassed {
				chartSuccessCount++
			}
		}

		if chartSuccessCount == len(deliveryVersion.Services) && taskSuccess {
			deliveryVersion.Status = setting.DeliveryVersionStatusSuccess
		} else {
			deliveryVersion.Status = setting.DeliveryVersionStatusFailed
		}
	}

	updateVersionStatusV2(deliveryVersion.Version, deliveryVersion.ProjectName, deliveryVersion.Status, deliveryVersion.Error)
	return taskDone, nil
}

func GetDeliveryVersionV2(args *commonrepo.DeliveryVersionV2Args, log *zap.SugaredLogger) (*commonmodels.DeliveryVersionV2, error) {
	versionData, err := commonrepo.NewDeliveryVersionV2Coll().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}
	return versionData, err
}

func GetDeliveryVersionDetailV2(args *commonrepo.DeliveryVersionV2Args, log *zap.SugaredLogger) (*commonmodels.DeliveryVersionV2, error) {
	version, err := commonrepo.NewDeliveryVersionV2Coll().Get(args)
	if err != nil {
		log.Errorf("get deliveryVersion error: %v", err)
		return nil, e.ErrGetDeliveryVersion
	}

	// order deploys by service name
	productTemplate, err := templaterepo.NewProductColl().Find(version.ProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to find product template %s, err: %v", version.ProjectName, err)
	}

	servicesOrder := productTemplate.Services
	if version.Production {
		servicesOrder = productTemplate.ProductionServices
	}

	i := 0
	serviceOrderMap := make(map[string]int)
	for _, serviceGroup := range servicesOrder {
		for _, service := range serviceGroup {
			serviceOrderMap[service] = i
			i++
		}
	}

	slices.SortStableFunc(version.Services, func(i, j *commonmodels.DeliveryVersionService) int {
		return cmp.Compare(serviceOrderMap[i.ServiceName], serviceOrderMap[j.ServiceName])
	})

	return version, nil
}

func CheckDeliveryImageStatus(version *commonmodels.DeliveryVersionV2, workflowTask *commonmodels.WorkflowTask, log *zap.SugaredLogger) (bool, error) {
	type distributeStatus struct {
		status config.Status
		err    error
	}
	distributeStatusMap := make(map[string]distributeStatus)

	for _, stage := range workflowTask.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == string(config.JobZadigDistributeImage) {
				jobSpec := &commonmodels.JobTaskFreestyleSpec{}
				if err := commonmodels.IToi(job.Spec, jobSpec); err != nil {
					log.Errorf("failed to parse job spec, err: %v", err)
					return false, fmt.Errorf("failed to parse job spec, err: %v", err)
				}

				for _, step := range jobSpec.Steps {
					if step.StepType == config.StepDistributeImage {
						stepSpec := &stepspec.StepImageDistributeSpec{}
						if err := commonmodels.IToi(step.Spec, stepSpec); err != nil {
							log.Errorf("failed to parse step spec, err: %v", err)
							return false, fmt.Errorf("failed to parse step spec, err: %v", err)
						}

						for _, target := range stepSpec.DistributeTarget {
							status := distributeStatus{
								status: job.Status,
								err:    nil,
							}

							if lo.Contains(config.FailedStatus(), job.Status) {
								status.err = fmt.Errorf("failed to build image distribute for service: %s, status: %s, err: %s ", target.ServiceName, job.Status, job.Error)
							}

							key := fmt.Sprintf("%s-%s", target.ServiceName, target.ServiceModule)
							if target.ServiceModule == "" {
								imageName := commonutil.ExtractImageName(target.TargetImage)
								key = fmt.Sprintf("%s-%s", target.ServiceName, imageName)
							}
							distributeStatusMap[key] = status
						}
					}
				}
			}
		}
	}

	changed := false
	for _, service := range version.Services {
		for _, image := range service.Images {
			key := fmt.Sprintf("%s-%s", service.ServiceName, image.ContainerName)
			distributeStatus, ok := distributeStatusMap[key]
			if ok && image.Status != distributeStatus.status {
				image.Status = distributeStatus.status
				if distributeStatus.err != nil {
					image.Error = distributeStatus.err.Error()
				}
				changed = true
			}
		}
	}

	return changed, nil
}

func DeleteDeliveryVersionV2(args *commonrepo.DeliveryVersionV2Args, log *zap.SugaredLogger) error {
	err := commonrepo.NewDeliveryVersionV2Coll().Delete(args.ID)
	if err != nil {
		log.Errorf("delete deliveryVersion error: %v", err)
		return e.ErrDeleteDeliveryVersion
	}
	return nil
}

type ListDeliveryVersionV2Args struct {
	Page         int    `form:"page"`
	PerPage      int    `form:"per_page"`
	TaskId       int    `form:"taskId"`
	ServiceName  string `form:"serviceName"`
	Verbosity    string `form:"verbosity"`
	ProjectName  string `form:"projectName"`
	WorkflowName string `form:"workflowName"`
}

func ListDeliveryVersionV2(args *ListDeliveryVersionV2Args, logger *zap.SugaredLogger) ([]*commonmodels.DeliveryVersionV2, int, error) {
	versionListArgs := new(commonrepo.DeliveryVersionV2Args)
	versionListArgs.ProjectName = args.ProjectName
	versionListArgs.WorkflowName = args.WorkflowName
	versionListArgs.TaskID = args.TaskId
	versionListArgs.PerPage = args.PerPage
	versionListArgs.Page = args.Page
	deliveryVersions, total, err := commonrepo.NewDeliveryVersionV2Coll().List(versionListArgs)
	if err != nil {
		return nil, 0, err
	}

	return deliveryVersions, total, nil
}

func ListDeliveryVersionV2Labels(projectName string) ([]string, error) {
	labels, err := commonrepo.NewDeliveryVersionV2Coll().ListLabels(projectName)
	if err != nil {
		return nil, err
	}

	return labels, nil
}

func GetDeliveryVersionV2LabelLatestVersion(projectName, label string) (*commonmodels.DeliveryVersionV2, error) {
	version, err := commonrepo.NewDeliveryVersionV2Coll().GetLabelLatestVersion(projectName, label)
	if err != nil {
		return nil, err
	}

	return version, nil
}

func generateDeliveryWorkflowName(projectName, version string) string {
	return fmt.Sprintf(deliveryVersionWorkflowV4NamingConvention, projectName, version)
}

func buildRegistryMap() (map[string]*commonmodels.RegistryNamespace, error) {
	registries, err := commonservice.ListRegistryNamespaces("", true, log.SugaredLogger())
	if err != nil {
		return nil, fmt.Errorf("failed to query registries")
	}
	ret := make(map[string]*commonmodels.RegistryNamespace)
	for _, singleRegistry := range registries {
		fullUrl := fmt.Sprintf("%s/%s", singleRegistry.RegAddr, singleRegistry.Namespace)
		fullUrl = strings.TrimSuffix(fullUrl, "/")
		u, _ := url.Parse(fullUrl)
		if len(u.Scheme) > 0 {
			fullUrl = strings.TrimPrefix(fullUrl, fmt.Sprintf("%s://", u.Scheme))
		}
		ret[fullUrl] = singleRegistry
	}
	return ret, nil
}

func makeChartFilesDir(project, versionName string) (string, error) {
	dirPath := getChartDir(project, versionName)
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create chart tgz dir for version: %s", versionName)
	}
	return dirPath, nil
}

func makeChartTGZFileDir(project, versionName string) (string, error) {
	dirPath := getChartTGZDir(project, versionName)
	if err := os.RemoveAll(dirPath); err != nil {
		if !os.IsExist(err) {
			return "", errors.Wrapf(err, "failed to claer dir for chart tgz files")
		}
	}
	err := os.MkdirAll(dirPath, 0777)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create chart tgz dir for version: %s", versionName)
	}
	return dirPath, nil
}

func getChartTGZDir(project, versionName string) string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "chart-tgz", project, versionName)
}

func getChartDir(project, versionName string) string {
	tmpDir := os.TempDir()
	return filepath.Join(tmpDir, "chart", project, versionName)
}

func fillChartUrl(charts []*DeliveryVersionPayloadChart, chartRepoName string) error {
	index, err := getIndexInfoFromChartRepo(chartRepoName)
	if err != nil {
		return err
	}
	chartMap := make(map[string]*DeliveryVersionPayloadChart)
	for _, chart := range charts {
		chartMap[chart.ChartName] = chart
	}

	for name, entries := range index.Entries {
		chart, ok := chartMap[name]
		if !ok {
			continue
		}
		if len(entries) == 0 {
			continue
		}

		for _, entry := range entries {
			if entry.Version == chart.ChartVersion {
				if len(entry.URLs) > 0 {
					chart.ChartUrl = entry.URLs[0]
				}
				break
			}
		}
	}
	return nil
}

func getIndexInfoFromChartRepo(chartRepoName string) (*repo.IndexFile, error) {
	chartRepo, err := getChartRepoData(chartRepoName)
	if err != nil {
		return nil, err
	}
	hClient, err := commonutil.NewHelmClient(chartRepo)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create chart repo client")
	}
	return hClient.FetchIndexYaml(commonutil.GeneHelmRepo(chartRepo))
}

func getChartRepoData(repoName string) (*commonmodels.HelmRepo, error) {
	return commonrepo.NewHelmRepoColl().Find(&commonrepo.HelmRepoFindOption{RepoName: repoName})
}

func CheckDeliveryVersion(projectName, deliveryVersionName string) error {
	if len(deliveryVersionName) == 0 {
		return e.ErrCheckDeliveryVersion.AddDesc("版本不能为空")
	}
	if len(projectName) == 0 {
		return e.ErrCheckDeliveryVersion.AddDesc("项目名称不能为空")
	}

	_, err := commonrepo.NewDeliveryVersionV2Coll().Get(&commonrepo.DeliveryVersionV2Args{
		ProjectName: projectName,
		Version:     deliveryVersionName,
	})
	if !mongodb.IsErrNoDocuments(err) {
		return e.ErrCheckDeliveryVersion.AddErr(fmt.Errorf("版本 %s 已存在", deliveryVersionName))
	}

	return nil
}

type DeliveryVariablesApplyArgs struct {
	GlobalVariables string                                 `json:"globalVariables,omitempty"`
	Services        []*commonmodels.DeliveryVersionService `json:"services"`
}

func ApplyDeliveryGlobalVariables(args *DeliveryVariablesApplyArgs, logger *zap.SugaredLogger) (interface{}, error) {
	ret := new(DeliveryVariablesApplyArgs)
	for _, service := range args.Services {
		mergedYaml, err := yamlutil.Merge([][]byte{[]byte(service.YamlContent), []byte(args.GlobalVariables)})
		if err != nil {
			logger.Errorf("failed to merge gobal variables for service: %s", service.ServiceName)
			return nil, errors.Wrapf(err, "failed to merge global variables for service: %s", service.ServiceName)
		}
		ret.Services = append(ret.Services, &commonmodels.DeliveryVersionService{
			ServiceName: service.ServiceName,
			YamlContent: string(mergedYaml),
		})
	}
	return ret, nil
}

type ChartVersionResp struct {
	ChartName        string `json:"chartName"`
	ChartVersion     string `json:"chartVersion"`
	NextChartVersion string `json:"nextChartVersion"`
	Url              string `json:"url"`
}

func GetChartVersion(chartName, chartRepoName string) ([]*ChartVersionResp, error) {
	index, err := getIndexInfoFromChartRepo(chartRepoName)
	if err != nil {
		return nil, err
	}

	chartNameList := strings.Split(chartName, ",")
	chartNameSet := sets.NewString(chartNameList...)
	existedChartSet := sets.NewString()

	ret := make([]*ChartVersionResp, 0)

	for name, entry := range index.Entries {
		if !chartNameSet.Has(name) {
			continue
		}
		if len(entry) == 0 {
			continue
		}
		latestEntry := entry[0]

		// generate suggested next chart version
		nextVersion := latestEntry.Version
		t, err := semver.Make(latestEntry.Version)
		if err != nil {
			log.Errorf("failed to parse current version: %s, err: %s", latestEntry.Version, err)
		} else {
			t.Patch = t.Patch + 1
			nextVersion = t.String()
		}

		ret = append(ret, &ChartVersionResp{
			ChartName:        name,
			ChartVersion:     latestEntry.Version,
			NextChartVersion: nextVersion,
		})
		existedChartSet.Insert(name)
	}

	for _, singleChartName := range chartNameList {
		if existedChartSet.Has(singleChartName) {
			continue
		}
		ret = append(ret, &ChartVersionResp{
			ChartName:        singleChartName,
			ChartVersion:     "",
			NextChartVersion: "1.0.0",
		})
	}

	return ret, nil
}

func DownloadDeliveryChart(projectName, version string, chartName string, log *zap.SugaredLogger) ([]byte, string, error) {
	filePath, err := preDownloadChart(projectName, version, chartName, log)
	if err != nil {
		return nil, "", err
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, "", err
	}

	return fileBytes, filepath.Base(filePath), err
}

func preDownloadChart(projectName, versionName, chartName string, log *zap.SugaredLogger) (string, error) {
	deliveryInfo, err := GetDeliveryVersionV2(&commonrepo.DeliveryVersionV2Args{
		ProjectName: projectName,
		Version:     versionName,
	}, log)
	if err != nil {
		return "", fmt.Errorf("failed to query delivery info")
	}

	service := &commonmodels.DeliveryVersionService{}
	for _, s := range deliveryInfo.Services {
		if s.ChartName == chartName {
			service = s
			break
		}
	}

	chartRepo, err := getChartRepoData(deliveryInfo.ChartRepoName)
	if err != nil {
		return "", err
	}

	// prepare chart data
	filePath, err := downloadChartV2(projectName, versionName, service.ChartName, service.ChartVersion, chartRepo, false)
	if err != nil {
		return "", err
	}

	return filePath, err
}

type ImageUrlDetail struct {
	ImageUrl         string
	Name             string
	SourceRegistryID string
	TargetRegistryID string
	Tag              string
	CustomTag        string
}

type ServiceImageDetails struct {
	ServiceName string
	Images      []*ImageUrlDetail
	Registries  []string
}

func updateValuesImage(service *commonmodels.DeliveryVersionService) ([]byte, error) {
	retValuesYaml := service.YamlContent

	imagePathSpecs := make([]map[string]string, 0)
	for _, image := range service.Images {
		imageSearchRule := &template.ImageSearchingRule{
			Repo:      image.ImagePath.Repo,
			Namespace: image.ImagePath.Namespace,
			Image:     image.ImagePath.Image,
			Tag:       image.ImagePath.Tag,
		}
		pattern := imageSearchRule.GetSearchingPattern()
		imagePathSpecs = append(imagePathSpecs, pattern)
	}

	for _, image := range service.Images {
		imageSearchRule := &template.ImageSearchingRule{
			Repo:      image.ImagePath.Repo,
			Namespace: image.ImagePath.Namespace,
			Image:     image.ImagePath.Image,
			Tag:       image.ImagePath.Tag,
		}
		pattern := imageSearchRule.GetSearchingPattern()

		// assign image to values.yaml
		replaceValuesMap, err := commonutil.AssignImageData(image.TargetImage, pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to pase image uri %s, err %s", image.TargetImage, err)
		}

		// replace image into final merged values.yaml
		retValuesYaml, err = commonutil.ReplaceImage(retValuesYaml, replaceValuesMap)
		if err != nil {
			return nil, err
		}
	}

	return []byte(retValuesYaml), nil
}

// func handleSingleChart(chartData *DeliveryChartData, product *commonmodels.Product, chartRepo *commonmodels.HelmRepo, dir string, globalVariables string,
// 	targetRegistry *commonmodels.RegistryNamespace, registryMap map[string]*commonmodels.RegistryNamespace) (*ServiceImageDetails, error) {
// 	serviceObj := chartData.ServiceObj

// 	deliveryChartPath, err := ensureChartFiles(chartData, product)
// 	if err != nil {
// 		return nil, err
// 	}

// 	valuesYamlData := make(map[string]interface{})
// 	valuesFilePath := filepath.Join(deliveryChartPath, setting.ValuesYaml)
// 	valueYamlContent, err := os.ReadFile(valuesFilePath)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to read values.yaml for service %s", serviceObj.ServiceName)
// 	}

// 	// write values.yaml file before load
// 	if len(chartData.ChartData.ValuesYamlContent) > 0 { // values.yaml was edited directly
// 		if err = yaml.Unmarshal([]byte(chartData.ChartData.ValuesYamlContent), map[string]interface{}{}); err != nil {
// 			log.Errorf("invalid yaml content, serviceName: %s, yamlContent: %s", serviceObj.ServiceName, chartData.ChartData.ValuesYamlContent)
// 			return nil, errors.Wrapf(err, "invalid yaml content for service: %s", serviceObj.ServiceName)
// 		}
// 		valueYamlContent = []byte(chartData.ChartData.ValuesYamlContent)
// 	} else if len(globalVariables) > 0 { // merge global variables
// 		valueYamlContent, err = yamlutil.Merge([][]byte{valueYamlContent, []byte(globalVariables)})
// 		if err != nil {
// 			return nil, errors.Wrapf(err, "failed to merge global variables for service: %s", serviceObj.ServiceName)
// 		}
// 	}

// 	// replace image url(registryUrl and imageTag)
// 	valueYamlContent, imageDetail, err := handleImageRegistry(valueYamlContent, chartData, targetRegistry, registryMap, chartData.ChartData.ImageData)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to handle image registry for service: %s", serviceObj.ServiceName)
// 	}

// 	err = yaml.Unmarshal(valueYamlContent, &valuesYamlData)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to unmarshal values.yaml for service %s", serviceObj.ServiceName)
// 	}

// 	// hold the currently running yaml data
// 	chartData.ValuesInEnv = valuesYamlData

// 	err = os.WriteFile(valuesFilePath, valueYamlContent, 0644)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to write values.yaml file for service %s", serviceObj.ServiceName)
// 	}

// 	//load chart info from local storage
// 	chartRequested, err := chartloader.Load(deliveryChartPath)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to load chart info, path %s", deliveryChartPath)
// 	}

// 	//set metadata
// 	chartRequested.Metadata.Name = chartData.ChartData.ServiceName
// 	chartRequested.Metadata.Version = chartData.ChartData.Version
// 	chartRequested.Metadata.AppVersion = chartData.ChartData.Version

// 	//create local chart package
// 	chartPackagePath, err := helm.CreateChartPackage(&helm.Chart{Chart: chartRequested}, dir)
// 	if err != nil {
// 		return nil, err
// 	}

// 	client, err := commonutil.NewHelmClient(chartRepo)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to create chart repo client, repoName: %s", chartRepo.RepoName)
// 	}

// 	proxy, err := commonutil.GenHelmChartProxy(chartRepo)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to generate helm chart proxy, repoName: %s", chartRepo.RepoName)
// 	}

// 	log.Debugf("pushing chart %s to %s...", filepath.Base(chartPackagePath), chartRepo.URL)
// 	err = client.PushChart(commonutil.GeneHelmRepo(chartRepo), chartPackagePath, proxy)
// 	if err != nil {
// 		return nil, errors.Wrapf(err, "failed to push chart: %s", chartPackagePath)
// 	}
// 	return imageDetail, nil
// }
