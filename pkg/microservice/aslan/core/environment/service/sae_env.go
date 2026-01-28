/*
Copyright 2021 The KodeRover Authors.

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
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sae "github.com/alibabacloud-go/sae-20190506/client"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/koderover/zadig/v2/pkg/util/converter"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	cloudservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/cloudservice"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/pkg/errors"
)

func ListSAEEnvs(userID, projectName string, envNames []string, production bool, log *zap.SugaredLogger) ([]*EnvResp, error) {
	envs, err := commonrepo.NewSAEEnvColl().List(&commonrepo.SAEEnvListOptions{
		ProjectName:         projectName,
		InEnvs:              envNames,
		Production:          &production,
		IsSortByProductName: true,
	})
	if err != nil {
		log.Errorf("Failed to list sae envs, err: %s", err)
		return nil, e.ErrListEnvs.AddDesc(err.Error())
	}

	var res []*EnvResp
	list, err := commonservice.ListFavorites(&mongodb.FavoriteArgs{
		UserID:      userID,
		ProductName: projectName,
		Type:        commonservice.FavoriteTypeEnv,
	})
	if err != nil {
		return nil, errors.Wrap(err, "list favorite environments")
	}
	// add personal favorite data in response
	favSet := sets.NewString(func() []string {
		var nameList []string
		for _, fav := range list {
			nameList = append(nameList, fav.Name)
		}
		return nameList
	}()...)

	for _, env := range envs {
		res = append(res, &EnvResp{
			ProjectName: projectName,
			Name:        env.EnvName,
			Production:  env.Production,
			UpdateTime:  env.UpdateTime,
			UpdateBy:    env.UpdateBy,
			Namespace:   env.NamespaceID,
			IsFavorite:  favSet.Has(env.EnvName),
		})
	}

	return res, nil
}

func GetSAEEnv(username, productName, envName string, production bool, log *zap.SugaredLogger) (*models.SAEEnv, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: productName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		log.Errorf("[User:%s][EnvName:%s][Product:%s] Product.FindByOwner error: %s", username, envName, productName, err)
		return nil, e.ErrGetEnv
	}

	return env, nil
}

func CreateSAEEnv(username string, env *models.SAEEnv, log *zap.SugaredLogger) error {
	envCheck1, err := commonrepo.NewSAEEnvColl().Find(&commonrepo.SAEEnvFindOptions{
		ProjectName:       env.ProjectName,
		EnvName:           env.EnvName,
		IgnoreNotFoundErr: true})
	if err != nil {
		err = fmt.Errorf("Failed to find sae env %s/%s, err: %s", env.ProjectName, env.EnvName, err)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}
	if envCheck1 != nil {
		err = fmt.Errorf("Envrionment %s/%s already exists", env.ProjectName, env.EnvName)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}
	envCheck2, err := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
		Name:              env.ProjectName,
		EnvName:           env.EnvName,
		IgnoreNotFoundErr: true})
	if err != nil {
		err = fmt.Errorf("Failed to find env %s/%s, err: %s", env.ProjectName, env.EnvName, err)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}
	if envCheck2 != nil {
		err = fmt.Errorf("Envrionment %s/%s already exists", env.ProjectName, env.EnvName)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}
	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}

	// add tags
	if len(env.Applications) > 0 {
		resourceIds := "["
		for _, app := range env.Applications {
			resourceIds += fmt.Sprintf(`"%s",`, app.AppID)
		}
		resourceIds = strings.TrimSuffix(resourceIds, ",") + "]"
		saeRequest := &sae.TagResourcesRequest{
			RegionId:     tea.String(env.RegionID),
			ResourceType: tea.String("application"),
			Tags:         tea.String(fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, env.ProjectName, setting.SAEZadigEnvTagKey, env.EnvName)),
			ResourceIds:  tea.String(resourceIds),
		}
		saeResp, err := saeClient.TagResources(saeRequest)
		if err != nil {
			err = fmt.Errorf("Failed to tag resources, err: %s", err)
			log.Error(err)
			return e.ErrCreateEnv.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("Failed to tag resources, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return e.ErrCreateEnv.AddErr(err)
		}
	}

	env.UpdateBy = username
	err = commonrepo.NewSAEEnvColl().Create(env)
	if err != nil {
		err = fmt.Errorf("Failed to create sae env, projectName:%s, envName: %s, error: %s", env.ProjectName, env.EnvName, err)
		log.Error(err)
		return e.ErrCreateEnv.AddErr(err)
	}

	return nil
}

func DeleteSAEEnv(username string, projectName, envName string, production bool, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("[User:%s][EnvName:%s][Product:%s] Find SAE Env error: %s", username, envName, projectName, err)
		log.Error(err)
		return e.ErrDeleteEnv.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrDeleteEnv.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrDeleteEnv.AddErr(err)
	}

	currentPage := int32(1)
	pageSize := int32(1000)
	for {
		resp, err := ListSAEApps(env.RegionID, env.NamespaceID, projectName, env.EnvName, production, "", false, currentPage, pageSize, log)
		if err != nil {
			err = fmt.Errorf("Failed to list sae apps, err: %s", err)
			log.Error(err)
			return e.ErrDeleteEnv.AddErr(err)
		}

		resourceIds := "["
		for _, app := range resp.Applications {
			resourceIds += fmt.Sprintf(`"%s",`, app.AppID)
		}
		resourceIds = strings.TrimSuffix(resourceIds, ",") + "]"
		saeRequest := &sae.UntagResourcesRequest{
			RegionId:     tea.String(env.RegionID),
			ResourceType: tea.String("application"),
			TagKeys:      tea.String(fmt.Sprintf(`["%s","%s","%s","%s"]`, setting.SAEZadigProjectTagKey, setting.SAEZadigEnvTagKey, setting.SAEZadigServiceTagKey, setting.SAEZadigServiceModuleTagKey)),
			ResourceIds:  tea.String(resourceIds),
		}
		saeResp, err := saeClient.UntagResources(saeRequest)
		if err != nil {
			err = fmt.Errorf("Failed to un tag resources, err: %s", err)
			log.Error(err)
			return e.ErrDeleteEnv.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("Failed to un tag resources, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return e.ErrDeleteEnv.AddErr(err)
		}

		if currentPage*pageSize >= resp.TotalSize {
			break
		}
		currentPage++
		pageSize += 1000
	}

	err = commonrepo.NewSAEEnvColl().Delete(projectName, envName)
	if err != nil {
		log.Errorf("[User:%s][EnvName:%s][Product:%s] delete sae env error: %s", username, envName, projectName, err)
		return e.ErrDeleteEnv
	}

	return nil
}

type ListSAEAppsResponse struct {
	Applications []*models.SAEApplication `json:"applications"`
	CurrentPage  int32                    `json:"current_page"`
	TotalSize    int32                    `json:"total_size"`
}

func ListSAEApps(regionID, namespace, projectName, envName string, production bool, appName string, isAddApp bool, currentPage, pageSize int32, log *zap.SugaredLogger) (*ListSAEAppsResponse, error) {
	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, regionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	tags := ""
	if !isAddApp {
		tags = fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, projectName, setting.SAEZadigEnvTagKey, envName)
	}
	saeRequest := &sae.ListApplicationsRequest{
		NamespaceId: tea.String(namespace),
		Tags:        tea.String(tags),
		AppName:     tea.String(appName),
		CurrentPage: tea.Int32(currentPage),
		PageSize:    tea.Int32(pageSize),
	}
	saeResp, err := saeClient.ListApplications(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to list applications, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to list applications, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	apps := make([]*models.SAEApplication, 0)
	for _, saeApp := range saeResp.Body.Data.Applications {
		tags := make([]*models.SAETag, 0)
		serviceName := ""
		serviceModule := ""
		for _, saeTag := range saeApp.Tags {
			tag := &models.SAETag{
				Key:   tea.StringValue(saeTag.Key),
				Value: tea.StringValue(saeTag.Value),
			}
			if tea.StringValue(saeTag.Key) == setting.SAEZadigServiceTagKey {
				serviceName = tea.StringValue(saeTag.Value)
			}
			if tea.StringValue(saeTag.Key) == setting.SAEZadigServiceModuleTagKey {
				serviceModule = tea.StringValue(saeTag.Value)
			}
			tags = append(tags, tag)
		}
		app := &models.SAEApplication{
			AppName:          tea.StringValue(saeApp.AppName),
			AppID:            tea.StringValue(saeApp.AppId),
			ImageUrl:         tea.StringValue(saeApp.ImageUrl),
			PackageUrl:       tea.StringValue(saeApp.PackageUrl),
			Tags:             tags,
			Instances:        tea.Int32Value(saeApp.Instances),
			RunningInstances: tea.Int32Value(saeApp.RunningInstances),
			Cpu:              tea.Int32Value(saeApp.Cpu),
			Mem:              tea.Int32Value(saeApp.Mem),
			ServiceName:      serviceName,
			ServiceModule:    serviceModule,
		}
		apps = append(apps, app)
	}

	resp := &ListSAEAppsResponse{
		Applications: apps,
		CurrentPage:  tea.Int32Value(saeResp.Body.CurrentPage),
		TotalSize:    tea.Int32Value(saeResp.Body.TotalSize),
	}

	return resp, nil
}

type SAENamespace struct {
	NameSpaceShortId     string `json:"namespace_short_id"`
	NamespaceName        string `json:"namespace_name"`
	NamespaceId          string `json:"namespace_id"`
	NamespaceDescription string `json:"namespace_description"`
}

func ListSAENamespaces(regionID string, log *zap.SugaredLogger) ([]*SAENamespace, error) {
	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, regionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	saeRequest := &sae.DescribeNamespacesRequest{
		CurrentPage: tea.Int32(1),
		PageSize:    tea.Int32(1000),
	}
	saeResp, err := saeClient.DescribeNamespaces(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to list namespace, err: %s", err)
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to list namespace, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, e.ErrListSAEApps.AddErr(err)
	}

	resp := make([]*SAENamespace, 0)
	for _, saeNs := range saeResp.Body.Data.Namespaces {
		ns := &SAENamespace{
			NamespaceId:          tea.StringValue(saeNs.NamespaceId),
			NameSpaceShortId:     tea.StringValue(saeNs.NameSpaceShortId),
			NamespaceName:        tea.StringValue(saeNs.NamespaceName),
			NamespaceDescription: tea.StringValue(saeNs.NamespaceDescription),
		}
		resp = append(resp, ns)
	}

	return resp, nil
}

func RestartSAEApp(projectName, envName string, production bool, appID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeRequest := &sae.RestartApplicationRequest{
		AppId: tea.String(appID),
	}
	saeResp, err := saeClient.RestartApplication(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to restart application %s, err: %s", appID, err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to restart application %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	return nil
}

func BindSAEAppToService(projectName, envName string, production bool, appID, serviceName, serviceModule string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	resourceIds, err := json.Marshal([]string{appID})
	if err != nil {
		err = fmt.Errorf("failed to bind sae application %s to service: json marshal failed, err: %s", appID, err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	// if service name and service module is empty, we delete the service binding tag for this app.
	if len(serviceName) == 0 && len(serviceModule) == 0 {
		tagKeys := []string{setting.SAEZadigServiceTagKey, setting.SAEZadigServiceModuleTagKey}
		tagKeyStr, err := json.Marshal(tagKeys)
		if err != nil {
			err = fmt.Errorf("failed to bind sae application %s to service: json marshal failed, err: %s", appID, err)
			log.Error(err)
			return e.ErrScaleService.AddErr(err)
		}

		saeRequest := &sae.UntagResourcesRequest{
			DeleteAll:    tea.Bool(false),
			RegionId:     tea.String(env.RegionID),
			ResourceIds:  tea.String(string(resourceIds)),
			ResourceType: tea.String("application"),
			TagKeys:      tea.String(string(tagKeyStr)),
		}

		saeResp, err := saeClient.UntagResources(saeRequest)
		if err != nil {
			err = fmt.Errorf("failed to unbind application %s to service, err: %s", appID, err)
			log.Error(err)
			return e.ErrScaleService.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("failed to unbind application %s to service, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return e.ErrScaleService.AddErr(err)
		}

		return nil
	}

	saeRequest := &sae.TagResourcesRequest{
		RegionId:     tea.String(env.RegionID),
		ResourceType: tea.String("application"),
		Tags:         tea.String(fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigServiceTagKey, serviceName, setting.SAEZadigServiceModuleTagKey, serviceModule)),
		ResourceIds:  tea.String(string(resourceIds)),
	}
	saeResp, err := saeClient.TagResources(saeRequest)
	if err != nil {
		err = fmt.Errorf("failed to bind application %s to service, err: %s", appID, err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to bind application %s to service, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	return nil
}

func RescaleSAEApp(projectName, envName string, production bool, appID string, Replicas int32, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	saeRequest := &sae.RescaleApplicationRequest{
		AppId:    tea.String(appID),
		Replicas: tea.Int32(Replicas),
	}
	saeResp, err := saeClient.RescaleApplication(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to rescale application %s, err: %s", appID, err)
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to rescale application %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return e.ErrScaleService.AddErr(err)
	}

	return nil
}

func RollbackSAEApp(ctx *internalhandler.Context, projectName, envName string, production bool, appID, versionID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrRollbackEnvServiceVersion.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrRollbackEnvServiceVersion.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrRollbackEnvServiceVersion.AddErr(err)
	}

	saeRequest := &sae.RollbackApplicationRequest{
		AppId:     tea.String(appID),
		VersionId: tea.String(versionID),
	}
	saeResp, err := saeClient.RollbackApplication(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to rollback application %s, err: %s", appID, err)
		log.Error(err)
		return e.ErrRollbackEnvServiceVersion.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to rollback application %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return e.ErrRollbackEnvServiceVersion.AddErr(err)
	}

	err = createSaeRollbackRecord(ctx, saeClient, projectName, envName, appID, versionID, env.Production)
	if err != nil {
		log.Error(errors.Wrap(err, "Failed to create sae rollback record"))
	}

	return nil
}

func createSaeRollbackRecord(ctx *internalhandler.Context, saeClient *sae.Client, projectName, envName, appID, versionID string, production bool) error {
	saeRequest := &sae.DescribeApplicationConfigRequest{
		AppId: tea.String(appID),
	}
	saeResp, err := saeClient.DescribeApplicationConfig(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to describe application %s config, err: %s", appID, err)
		log.Error(err)
		return err
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to describe application %s config, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return err
	}

	originSaeApp := &models.SAEApplication{
		AppName:    tea.StringValue(saeResp.Body.Data.AppName),
		AppID:      tea.StringValue(saeResp.Body.Data.AppId),
		ImageUrl:   tea.StringValue(saeResp.Body.Data.ImageUrl),
		PackageUrl: tea.StringValue(saeResp.Body.Data.PackageUrl),
		Instances:  tea.Int32Value(saeResp.Body.Data.Replicas),
		Cpu:        tea.Int32Value(saeResp.Body.Data.Cpu),
		Mem:        tea.Int32Value(saeResp.Body.Data.Memory),
	}

	saeRequest = &sae.DescribeApplicationConfigRequest{
		AppId:     tea.String(appID),
		VersionId: tea.String(versionID),
	}
	saeResp, err = saeClient.DescribeApplicationConfig(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to describe application %s config, versionID: %s, err: %s", appID, versionID, err)
		log.Error(err)
		return err
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to describe application %s config, versionID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, versionID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return err
	}

	updateSaeApp := &models.SAEApplication{
		AppName:    tea.StringValue(saeResp.Body.Data.AppName),
		AppID:      tea.StringValue(saeResp.Body.Data.AppId),
		ImageUrl:   tea.StringValue(saeResp.Body.Data.ImageUrl),
		PackageUrl: tea.StringValue(saeResp.Body.Data.PackageUrl),
		Instances:  tea.Int32Value(saeResp.Body.Data.Replicas),
		Cpu:        tea.Int32Value(saeResp.Body.Data.Cpu),
		Mem:        tea.Int32Value(saeResp.Body.Data.Memory),
	}

	rollbackRecord := &commonmodels.EnvInfo{
		ProjectName:   projectName,
		EnvName:       envName,
		EnvType:       config.EnvTypeSae,
		Production:    production,
		Operation:     config.EnvOperationRollback,
		OperationType: config.EnvOperationTypeSae,
		ServiceName:   originSaeApp.AppName,
		ServiceType:   config.ServiceTypeSae,
		OriginSaeApp:  originSaeApp,
		UpdateSaeApp:  updateSaeApp,
	}
	err = mongodb.NewEnvInfoColl().Create(ctx, rollbackRecord)
	if err != nil {
		err = fmt.Errorf("Failed to create sae rollback record, projectName: %s, envName: %s, appName %s, error: %s", projectName, envName, originSaeApp.AppName, err)
		log.Error(err)
		return err
	}

	return nil
}

type SAEAppVersion struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	CreateTime      string `json:"create_time"`
	BuildPackageUrl string `json:"build_package_url"`
	WarUrl          string `json:"war_url"`
}

func ListSAEAppVersions(projectName, envName string, production bool, appID string, log *zap.SugaredLogger) ([]*SAEAppVersion, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return nil, e.ErrListEnvServiceVersions.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, e.ErrListEnvServiceVersions.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return nil, e.ErrRollbackEnvServiceVersion.AddErr(err)
	}

	saeRequest := &sae.ListAppVersionsRequest{
		AppId: tea.String(appID),
	}
	saeResp, err := saeClient.ListAppVersions(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to list application %s version, err: %s", appID, err)
		log.Error(err)
		return nil, e.ErrListEnvServiceVersions.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to list application %s version, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, e.ErrListEnvServiceVersions.AddErr(err)
	}

	resp := make([]*SAEAppVersion, 0)
	for _, saeVersion := range saeResp.Body.Data {
		version := &SAEAppVersion{
			ID:              tea.StringValue(saeVersion.Id),
			Type:            tea.StringValue(saeVersion.Type),
			CreateTime:      tea.StringValue(saeVersion.CreateTime),
			BuildPackageUrl: tea.StringValue(saeVersion.BuildPackageUrl),
			WarUrl:          tea.StringValue(saeVersion.WarUrl),
		}
		resp = append(resp, version)
	}

	return resp, nil
}

type SAEAppInstance struct {
	InstanceId                string `json:"instance_id"`
	GroupID                   string `json:"group_id"`
	InstanceHealthStatus      string `json:"instance_health_status"`
	InstanceContainerStatus   string `json:"instance_container_status"`
	InstanceContainerRestarts int64  `json:"instance_container_restarts"`
	InstanceContainerIp       string `json:"instance_container_ip"`
	Eip                       string `json:"eip"`
	ImageURL                  string `json:"image_url"`
	PackageVersion            string `json:"package_version"`
	CreateTimeStamp           int64  `json:"create_timestamp"`
	FinishTimeStamp           int64  `json:"finish_timestamp"`
}

type SAEAppGroup struct {
	GroupType        int32             `json:"group_type"`
	RunningInstances int32             `json:"running_instances"`
	Replicas         int32             `json:"replicas"`
	GroupId          string            `json:"group_id"`
	GroupName        string            `json:"group_name"`
	PackageType      string            `json:"package_type"`
	PackageVersion   string            `json:"package_version"`
	PackageUrl       string            `json:"package_url"`
	ImageUrl         string            `json:"image_url"`
	Instances        []*SAEAppInstance `json:"instances"`
}

func ListSAEAppInstances(projectName, envName string, production bool, appID string, log *zap.SugaredLogger) ([]*SAEAppGroup, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return nil, e.ErrListServicePod.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, e.ErrListServicePod.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return nil, e.ErrListServicePod.AddErr(err)
	}

	saeRequest := &sae.DescribeApplicationGroupsRequest{
		AppId: tea.String(appID),
	}
	saeResp, err := saeClient.DescribeApplicationGroups(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to list application %s group, err: %s", appID, err)
		log.Error(err)
		return nil, e.ErrListServicePod.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to list application %s version, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, e.ErrListServicePod.AddErr(err)
	}

	resp := make([]*SAEAppGroup, 0)
	for _, saeGroup := range saeResp.Body.Data {
		group := &SAEAppGroup{
			GroupType:        tea.Int32Value(saeGroup.GroupType),
			RunningInstances: tea.Int32Value(saeGroup.RunningInstances),
			Replicas:         tea.Int32Value(saeGroup.Replicas),
			GroupId:          tea.StringValue(saeGroup.GroupId),
			GroupName:        tea.StringValue(saeGroup.GroupName),
			PackageType:      tea.StringValue(saeGroup.PackageType),
			PackageVersion:   tea.StringValue(saeGroup.PackageVersion),
			PackageUrl:       tea.StringValue(saeGroup.PackageUrl),
			ImageUrl:         tea.StringValue(saeGroup.ImageUrl),
		}

		saeRequest := &sae.DescribeApplicationInstancesRequest{
			AppId:       tea.String(appID),
			GroupId:     tea.String(group.GroupId),
			CurrentPage: tea.Int32(1),
			PageSize:    tea.Int32(1000),
		}
		saeResp, err := saeClient.DescribeApplicationInstances(saeRequest)
		if err != nil {
			err = fmt.Errorf("Failed to list application %s instance, err: %s", appID, err)
			log.Error(err)
			return nil, e.ErrListServicePod.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("Failed to list application %s instance, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return nil, e.ErrListServicePod.AddErr(err)
		}

		instances := make([]*SAEAppInstance, 0)
		for _, saeInstance := range saeResp.Body.Data.Instances {
			instance := &SAEAppInstance{
				InstanceId:                tea.StringValue(saeInstance.InstanceId),
				GroupID:                   tea.StringValue(saeInstance.GroupId),
				InstanceHealthStatus:      tea.StringValue(saeInstance.InstanceHealthStatus),
				InstanceContainerStatus:   tea.StringValue(saeInstance.InstanceContainerStatus),
				InstanceContainerRestarts: tea.Int64Value(saeInstance.InstanceContainerRestarts),
				InstanceContainerIp:       tea.StringValue(saeInstance.InstanceContainerIp),
				Eip:                       tea.StringValue(saeInstance.Eip),
				ImageURL:                  tea.StringValue(saeInstance.ImageUrl),
				PackageVersion:            tea.StringValue(saeInstance.PackageVersion),
				CreateTimeStamp:           tea.Int64Value(saeInstance.CreateTimeStamp),
				FinishTimeStamp:           tea.Int64Value(saeInstance.FinishTimeStamp),
			}
			instances = append(instances, instance)
		}
		group.Instances = instances

		resp = append(resp, group)
	}

	return resp, nil
}

func RestartSAEAppInstance(projectName, envName string, production bool, appID, instanceID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	saeRequest := &sae.RestartInstancesRequest{
		AppId:       tea.String(appID),
		InstanceIds: tea.String(instanceID),
	}
	saeResp, err := saeClient.RestartInstances(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to restart instance, appID: %s, instanceID: %s, err: %s", appID, instanceID, err)
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to restart instance, appID: %s, instanceID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, instanceID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return e.ErrRestartService.AddErr(err)
	}

	return nil
}

func ListSAEChangeOrder(projectName, envName string, production bool, appID string, page, perPage int, log *zap.SugaredLogger) (interface{}, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil 	{
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	saeRequest := &sae.ListChangeOrdersRequest{
		AppId:       tea.String(appID),
		CurrentPage: tea.Int32(int32(page)),
		PageSize:    tea.Int32(int32(perPage)),
	}
	saeResp, err := saeClient.ListChangeOrdersWithOptions(saeRequest, generateCNCookie(), &teautil.RuntimeOptions{})
	if err != nil {
		err = fmt.Errorf("failed to get change order list, appID: %s, err: %s", appID, err)
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to get change order list, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	return converter.ConvertToSnakeCase(saeResp.Body.Data)
}

func GetSAEChangeOrder(projectName, envName, appID, orderID string, log *zap.SugaredLogger) (interface{}, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeRequest := &sae.DescribeChangeOrderRequest{ChangeOrderId: tea.String(orderID)}
	saeResp, err := saeClient.DescribeChangeOrderWithOptions(saeRequest, generateCNCookie(), &teautil.RuntimeOptions{})
	if err != nil {
		err = fmt.Errorf("failed to get change order detail, orderID: %s, appID: %s, err: %s", orderID, appID, err)
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to get change order detail, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return "", e.ErrGetService.AddErr(err)
	}

	return converter.ConvertToSnakeCase(saeResp.Body.Data)
}

func AbortSAEChangeOrder(projectName, envName string, production bool, appID, orderID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return err
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return err
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return err
	}

	saeRequest := &sae.AbortChangeOrderRequest{ChangeOrderId: tea.String(orderID)}
	saeResp, err := saeClient.AbortChangeOrder(saeRequest)
	if err != nil {
		err = fmt.Errorf("failed to abort change order, orderID: %s, appID: %s, err: %s", orderID, appID, err)
		log.Error(err)
		return err
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to abort change order, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return err
	}

	return nil
}

func RollbackSAEChangeOrder(ctx *internalhandler.Context, projectName, envName string, production bool, appID, orderID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return err
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return err
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return err
	}

	saeRequest := &sae.AbortAndRollbackChangeOrderRequest{ChangeOrderId: tea.String(orderID)}
	saeResp, err := saeClient.AbortAndRollbackChangeOrder(saeRequest)
	if err != nil {
		err = fmt.Errorf("failed to rollback change order, orderID: %s, appID: %s, err: %s", orderID, appID, err)
		log.Error(err)
		return err
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to rollback change order, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return err
	}

	app, err := getSaeApplication(saeClient, appID)
	if err != nil {
		err = errors.Wrapf(err, "Failed to get sae application %s", appID)
		log.Error(err)
		return err
	}

	rollbackRecord := &commonmodels.EnvInfo{
		ProjectName:   projectName,
		EnvName:       envName,
		EnvType:       config.EnvTypeSae,
		Production:    env.Production,
		Operation:     config.EnvOperationRollback,
		OperationType: config.EnvOperationTypeSaeChangeOrder,
		ServiceName:   app.AppName,
		ServiceType:   config.ServiceTypeSae,
	}
	err = mongodb.NewEnvInfoColl().Create(ctx, rollbackRecord)
	if err != nil {
		err = fmt.Errorf("Failed to create sae rollback record, projectName: %s, envName: %s, appName %s, error: %s", projectName, envName, app.AppName, err)
		log.Error(err)
		return err
	}

	return nil
}

func ConfirmSAEPipelineBatch(projectName, envName string, production bool, appID, pipelineID string, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return err
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return err
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return err
	}

	saeRequest := &sae.ConfirmPipelineBatchRequest{
		PipelineId: tea.String(pipelineID),
		Confirm:    tea.Bool(true),
	}
	saeResp, err := saeClient.ConfirmPipelineBatch(saeRequest)
	if err != nil {
		err = fmt.Errorf("failed to rollback change order, pipelineID: %s, appID: %s, err: %s", pipelineID, appID, err)
		log.Error(err)
		return err
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to rollback change order, appID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return err
	}

	return nil
}

func GetSAEPipeline(projectName, envName string, production bool, appID, pipelineID string, log *zap.SugaredLogger) (interface{}, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return nil, err
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("failed to find default sae, err: %s", err)
		log.Error(err)
		return nil, err
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("failed to create sae client, err: %s", err)
		log.Error(err)
		return nil, err
	}

	saeRequest := &sae.DescribePipelineRequest{PipelineId: tea.String(pipelineID)}
	saeResp, err := saeClient.DescribePipelineWithOptions(saeRequest, generateCNCookie(), &teautil.RuntimeOptions{})
	if err != nil {
		err = fmt.Errorf("failed to get pipeline, pipelineID: %s, appID: %s, err: %s", pipelineID, appID, err)
		log.Error(err)
		return nil, err
	}

	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("failed to get pipeline, pipelineID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", pipelineID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, err
	}

	return converter.ConvertToSnakeCase(saeResp.Body.Data)
}

func GetSAEAppInstanceLog(projectName, envName string, production bool, appID, instanceID string, log *zap.SugaredLogger) (string, error) {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	saeRequest := &sae.DescribeInstanceLogRequest{
		InstanceId: tea.String(instanceID),
	}
	saeResp, err := saeClient.DescribeInstanceLog(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to get instance log, appID: %s, instanceID: %s, err: %s", appID, instanceID, err)
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		err = fmt.Errorf("Failed to get instance log, appID: %s, instanceID: %s, statusCode: %d, code: %s, errCode: %s, message: %s", appID, instanceID, tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return "", e.ErrQueryContainerLogs.AddErr(err)
	}

	return tea.StringValue(saeResp.Body.Data), nil
}

type AddSAEAppToEnvRequest struct {
	AppIDs []string `json:"app_ids"`
}

func AddSAEAppToEnv(username string, projectName, envName string, production bool, req *AddSAEAppToEnvRequest, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrAddSAEAppToEnv.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrAddSAEAppToEnv.AddErr(err)
	}
	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrAddSAEAppToEnv.AddErr(err)
	}

	if len(req.AppIDs) > 0 {
		resourceIds, err := json.Marshal(req.AppIDs)
		if err != nil {
			err = fmt.Errorf("failed to tag resources: marshal app_ids failed, err: %s", err)
			log.Error(err)
			return e.ErrAddSAEAppToEnv.AddErr(err)
		}

		saeRequest := &sae.TagResourcesRequest{
			RegionId:     tea.String(env.RegionID),
			ResourceType: tea.String("application"),
			Tags:         tea.String(fmt.Sprintf(`[{"Key":"%s","Value":"%s"}, {"Key":"%s","Value":"%s"}]`, setting.SAEZadigProjectTagKey, env.ProjectName, setting.SAEZadigEnvTagKey, env.EnvName)),
			ResourceIds:  tea.String(string(resourceIds)),
		}
		saeResp, err := saeClient.TagResources(saeRequest)
		if err != nil {
			err = fmt.Errorf("Failed to tag resources, err: %s", err)
			log.Error(err)
			return e.ErrAddSAEAppToEnv.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("Failed to tag resources, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return e.ErrAddSAEAppToEnv.AddErr(err)
		}
	}

	return nil
}

type DelSAEAppFromEnvRequest struct {
	AppIDs []string `json:"app_ids"`
}

func DelSAEAppFromEnv(username string, projectName, envName string, production bool, req *DelSAEAppFromEnvRequest, log *zap.SugaredLogger) error {
	opt := &commonrepo.SAEEnvFindOptions{ProjectName: projectName, EnvName: envName, Production: &production}
	env, err := commonrepo.NewSAEEnvColl().Find(opt)
	if err != nil {
		err = fmt.Errorf("Failed to find SAE env, projectName: %s, envName: %s, error: %s", projectName, envName, err)
		log.Error(err)
		return e.ErrDelSAEAppFromEnv.AddErr(err)
	}

	saeModel, err := commonrepo.NewCloudServiceColl().FindDefault(context.Background(), setting.CloudServiceTypeSAE)
	if err != nil {
		err = fmt.Errorf("Failed to find default sae, err: %s", err)
		log.Error(err)
		return e.ErrDelSAEAppFromEnv.AddErr(err)
	}
	saeClient, err := cloudservice.NewSAEClient(saeModel, env.RegionID)
	if err != nil {
		err = fmt.Errorf("Failed to create sae client, err: %s", err)
		log.Error(err)
		return e.ErrDelSAEAppFromEnv.AddErr(err)
	}

	if len(req.AppIDs) > 0 {
		resourceIds := "["
		for _, app := range req.AppIDs {
			resourceIds += fmt.Sprintf(`"%s",`, app)
		}
		resourceIds = strings.TrimSuffix(resourceIds, ",") + "]"
		saeRequest := &sae.UntagResourcesRequest{
			RegionId:     tea.String(env.RegionID),
			ResourceType: tea.String("application"),
			TagKeys:      tea.String(fmt.Sprintf(`["%s","%s","%s","%s"]`, setting.SAEZadigProjectTagKey, setting.SAEZadigEnvTagKey, setting.SAEZadigServiceTagKey, setting.SAEZadigServiceModuleTagKey)),
			ResourceIds:  tea.String(resourceIds),
		}
		saeResp, err := saeClient.UntagResources(saeRequest)
		if err != nil {
			err = fmt.Errorf("Failed to un tag resources, err: %s", err)
			log.Error(err)
			return e.ErrDelSAEAppFromEnv.AddErr(err)
		}
		if !tea.BoolValue(saeResp.Body.Success) {
			err = fmt.Errorf("Failed to un tag resources, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Code), tea.ToString(saeResp.Body.ErrorCode), tea.ToString(saeResp.Body.Message))
			log.Error(err)
			return e.ErrDelSAEAppFromEnv.AddErr(err)
		}
	}

	return nil
}

func generateCNCookie() map[string]*string {
	cnCookie := "aliyun_lang=zh"
	return map[string]*string{
		"cookie": tea.String(cnCookie),
	}
}

func getSaeApplication(saeClient *sae.Client, appId string) (*commonmodels.SAEApplication, error) {
	saeRequest := &sae.GetApplicationRequest{
		AppId: tea.String(appId),
	}
	saeResp, err := saeClient.GetApplication(saeRequest)
	if err != nil {
		err = fmt.Errorf("Failed to un tag resources, err: %s", err)
		log.Error(err)
		return nil, err
	}

	if tea.StringValue(saeResp.Body.Message) != "success" {
		err = fmt.Errorf("Failed to get application, statusCode: %d, message: %s", tea.Int32Value(saeResp.StatusCode), tea.ToString(saeResp.Body.Message))
		log.Error(err)
		return nil, err
	}

	app := &commonmodels.SAEApplication{
		AppName:          tea.StringValue(saeResp.Body.Application.AppName),
		AppID:            tea.StringValue(saeResp.Body.Application.AppId),
		Instances:        tea.Int32Value(saeResp.Body.Application.Instances),
		RunningInstances: tea.Int32Value(saeResp.Body.Application.RunningInstances),
		Cpu:              tea.Int32Value(saeResp.Body.Application.Cpu),
		Mem:              tea.Int32Value(saeResp.Body.Application.Mem),
	}

	return app, nil
}
