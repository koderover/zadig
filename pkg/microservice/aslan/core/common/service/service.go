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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	templ "text/template"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	templatemodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models/template"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/webhook"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/codehost"
	"github.com/koderover/zadig/pkg/shared/poetry"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/converter"
	yamlutil "github.com/koderover/zadig/pkg/util/yaml"
)

type yamlPreview struct {
	Kind string `json:"kind"`
}

type ServiceTmplResp struct {
	Data  []*ServiceProductMap `json:"data"`
	Total int                  `json:"total"`
}

type ServiceTmplBuildObject struct {
	ServiceTmplObject *ServiceTmplObject  `json:"pm_service_tmpl"`
	Build             *commonmodels.Build `json:"build"`
}

type ServiceTmplObject struct {
	ProductName  string                        `json:"product_name"`
	ServiceName  string                        `json:"service_name"`
	Visibility   string                        `json:"visibility"`
	Revision     int64                         `json:"revision"`
	Type         string                        `json:"type"`
	Username     string                        `json:"username"`
	EnvConfigs   []*commonmodels.EnvConfig     `json:"env_configs"`
	EnvStatuses  []*commonmodels.EnvStatus     `json:"env_statuses,omitempty"`
	From         string                        `json:"from,omitempty"`
	HealthChecks []*commonmodels.PmHealthCheck `json:"health_checks"`
}

type ServiceProductMap struct {
	Service          string                    `json:"service_name"`
	Source           string                    `json:"source"`
	Type             string                    `json:"type"`
	Product          []string                  `json:"product"`
	ProductName      string                    `json:"product_name"`
	Containers       []*commonmodels.Container `json:"containers,omitempty"`
	Visibility       string                    `json:"visibility,omitempty"`
	CodehostID       int                       `json:"codehost_id"`
	RepoOwner        string                    `json:"repo_owner"`
	RepoName         string                    `json:"repo_name"`
	RepoUUID         string                    `json:"repo_uuid"`
	BranchName       string                    `json:"branch_name"`
	LoadPath         string                    `json:"load_path"`
	LoadFromDir      bool                      `json:"is_dir"`
	GerritRemoteName string                    `json:"gerrit_remote_name,omitempty"`
}

var (
	imageParseRegex = regexp.MustCompile(`(?P<repo>.+/)?(?P<image>[^:]+){1}(:)?(?P<tag>.+)?`)
	presetPatterns  = []map[string]string{
		{setting.PathSearchComponentImage: "image.repository", setting.PathSearchComponentTag: "image.tag"},
		{setting.PathSearchComponentImage: "image"},
	}
)

// ListServiceTemplate 列出服务模板
func ListServiceTemplate(productName string, log *zap.SugaredLogger) (*ServiceTmplResp, error) {
	var err error
	resp := new(ServiceTmplResp)
	resp.Data = make([]*ServiceProductMap, 0)
	productTmpl, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Can not find project %s, error: %s", productName, err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	services, err := commonrepo.NewServiceColl().ListMaxRevisionsForServices(productTmpl.AllServiceInfos(), "")

	if err != nil {
		log.Errorf("Failed to list services by %+v, err: %s", productTmpl.AllServiceInfos(), err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	serviceToProject, err := GetServiceInvolvedProjects(services, "")
	if err != nil {
		log.Errorf("Failed to get service involved projects, err: %s", err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	for _, serviceObject := range services {
		// FIXME: 兼容老数据，想办法干掉这个
		if serviceObject.Source == setting.SourceFromGitlab && serviceObject.CodehostID == 0 {
			gitlabAddress, err := GetGitlabAddress(serviceObject.SrcPath)
			if err != nil {
				log.Errorf("无法从原有数据中恢复加载信息, GetGitlabAddr failed err: %+v", err)
				return nil, e.ErrListTemplate.AddDesc(err.Error())
			}

			details, err := codehost.ListCodehostDetial()
			if err != nil {
				log.Errorf("无法从原有数据中恢复加载信息, listCodehostDetail failed err: %+v", err)
				return nil, e.ErrListTemplate.AddDesc(err.Error())
			}
			for _, detail := range details {
				if strings.Contains(detail.Address, gitlabAddress) {
					serviceObject.CodehostID = detail.ID
				}
			}
			_, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(serviceObject.SrcPath)
			if err != nil {
				log.Errorf("Failed to load info from url: %s, the error is: %+v", serviceObject.SrcPath, err)
				return nil, e.ErrListTemplate.AddDesc(fmt.Sprintf("Failed to load info from url: %s, the error is: %+v", serviceObject.SrcPath, err))
			}
			// 万一codehost被删了，找不到
			if serviceObject.CodehostID == 0 {
				log.Errorf("Failed to find the old code host info")
				return nil, e.ErrListTemplate.AddDesc("无法找到原有的codehost信息，请确认codehost仍然存在")
			}
			serviceObject.RepoOwner = owner
			serviceObject.RepoName = r
			serviceObject.BranchName = branch
			serviceObject.LoadPath = loadPath
			serviceObject.LoadFromDir = true
		} else if serviceObject.Source == setting.SourceFromGithub && serviceObject.RepoName == "" {
			address, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(serviceObject.SrcPath)
			if err != nil {
				return nil, err
			}

			detail, err := codehost.GetCodeHostInfo(
				&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
			if err != nil {
				log.Errorf("get github codeHostInfo failed, err:%v", err)
				return nil, err
			}
			serviceObject.CodehostID = detail.ID
			serviceObject.RepoOwner = owner
			serviceObject.RepoName = r
			serviceObject.BranchName = branch
			serviceObject.LoadPath = loadPath
			serviceObject.LoadFromDir = true
		}

		spmap := &ServiceProductMap{
			Service:          serviceObject.ServiceName,
			Type:             serviceObject.Type,
			Source:           serviceObject.Source,
			ProductName:      serviceObject.ProductName,
			Containers:       serviceObject.Containers,
			Product:          []string{productName},
			Visibility:       serviceObject.Visibility,
			CodehostID:       serviceObject.CodehostID,
			RepoOwner:        serviceObject.RepoOwner,
			RepoName:         serviceObject.RepoName,
			RepoUUID:         serviceObject.RepoUUID,
			BranchName:       serviceObject.BranchName,
			LoadFromDir:      serviceObject.LoadFromDir,
			LoadPath:         serviceObject.LoadPath,
			GerritRemoteName: serviceObject.GerritRemoteName,
		}

		if _, ok := serviceToProject[serviceObject.ServiceName]; ok {
			spmap.Product = serviceToProject[serviceObject.ServiceName]
		}

		resp.Data = append(resp.Data, spmap)
	}

	return resp, nil
}

// ListWorkloadTemplate 列出实例模板
func ListWorkloadTemplate(productName, envName string, log *zap.SugaredLogger) (*ServiceTmplResp, error) {
	var err error
	resp := new(ServiceTmplResp)
	resp.Data = make([]*ServiceProductMap, 0)
	productTmpl, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		log.Errorf("Can not find project %s, error: %s", productName, err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	services, err := commonrepo.NewServiceColl().ListExternalWorkloadsBy(productName, envName)
	if err != nil {
		log.Errorf("Failed to list external services by %+v, err: %s", productTmpl.AllServiceInfos(), err)
		return resp, e.ErrListTemplate.AddDesc(err.Error())
	}

	for _, serviceObject := range services {
		spmap := &ServiceProductMap{
			Service:     serviceObject.ServiceName,
			Type:        serviceObject.Type,
			Source:      serviceObject.Source,
			ProductName: serviceObject.ProductName,
			Containers:  serviceObject.Containers,
		}
		resp.Data = append(resp.Data, spmap)
	}

	return resp, nil
}

// GetServiceInvolvedProjects returns a map, key is a service name, value is a list of all projects which are using this service.
// The given services must come from same project to make sure all service names are unique.
func GetServiceInvolvedProjects(services []*commonmodels.Service, skipProject string) (map[string][]string, error) {
	serviceMap := make(map[string]sets.String)
	serviceToOwner := make(map[string]string)
	var publicServiceInfos []*templatemodels.ServiceInfo
	for _, s := range services {
		serviceMap[s.ServiceName] = sets.NewString(s.ProductName)
		serviceToOwner[s.ServiceName] = s.ProductName

		if s.Visibility == setting.PublicService {
			publicServiceInfos = append(publicServiceInfos, &templatemodels.ServiceInfo{
				Name:  s.ServiceName,
				Owner: s.ProductName,
			})
		}
	}

	projects, err := templaterepo.NewProductColl().ListWithOption(&templaterepo.ProductListOpt{ContainSharedServices: publicServiceInfos})
	if err != nil {
		return nil, err
	}

	for _, project := range projects {
		for _, service := range project.SharedServices {
			// skip service which is not in the list or the owner is different
			if serviceToOwner[service.Name] != service.Owner {
				continue
			}
			serviceMap[service.Name] = serviceMap[service.Name].Insert(project.ProductName)
		}
	}

	res := make(map[string][]string)
	for k, v := range serviceMap {
		v.Delete(skipProject)
		res[k] = v.List()
	}
	return res, nil
}

func GetServiceTemplate(serviceName, serviceType, productName, excludeStatus string, revision int64, log *zap.SugaredLogger) (*commonmodels.Service, error) {
	opt := &commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		Type:        serviceType,
		Revision:    revision,
		ProductName: productName,
	}
	if excludeStatus != "" {
		opt.ExcludeStatus = excludeStatus
	}

	resp, err := commonrepo.NewServiceColl().Find(opt)
	if err != nil {
		errMsg := fmt.Sprintf("[ServiceTmpl.Find] %s error: %v", serviceName, err)
		log.Error(errMsg)
		return resp, e.ErrGetTemplate.AddDesc(errMsg)
	}

	if resp.Source == setting.SourceFromGitlab && resp.RepoName == "" {
		if gitlabAddress, err := GetGitlabAddress(resp.SrcPath); err == nil {
			if details, err := codehost.ListCodehostDetial(); err == nil {
				for _, detail := range details {
					if strings.Contains(detail.Address, gitlabAddress) {
						resp.GerritCodeHostID = detail.ID
						resp.CodehostID = detail.ID
					}
				}
				_, owner, r, branch, loadPath, pathType, err := GetOwnerRepoBranchPath(resp.SrcPath)
				if err != nil {
					log.Errorf("Failed to load info from url: %s, the error is: %+v", resp.SrcPath, err)
					return nil, e.ErrGetService.AddDesc(fmt.Sprintf("Failed to load info from url: %s, the error is: %+v", resp.SrcPath, err))
				}
				// 万一codehost被删了，找不到
				if resp.CodehostID == 0 {
					log.Errorf("Failed to find the old code host info")
					return nil, e.ErrListTemplate.AddDesc("无法找到原有的codehost信息，请确认codehost仍然存在")
				}
				resp.RepoOwner = owner
				resp.RepoName = r
				resp.BranchName = branch
				resp.LoadPath = loadPath
				resp.LoadFromDir = pathType == "tree"
				return resp, nil
			}
			errMsg := fmt.Sprintf("[ServiceTmpl.Find]  ListCodehostDetail %s error: %v", serviceName, err)
			log.Error(errMsg)
		} else {
			errMsg := fmt.Sprintf("[ServiceTmpl.Find]  GetGitlabAddress %s error: %v", serviceName, err)
			log.Error(errMsg)
		}

	} else if resp.Source == setting.SourceFromGithub && resp.GerritCodeHostID == 0 {
		address, owner, r, branch, loadPath, _, err := GetOwnerRepoBranchPath(resp.SrcPath)
		if err != nil {
			return nil, err
		}

		detail, err := codehost.GetCodeHostInfo(
			&codehost.Option{CodeHostType: poetry.GitHubProvider, Address: address, Namespace: owner})
		if err != nil {
			log.Errorf("get github codeHostInfo failed, err:%v", err)
			return nil, err
		}
		resp.CodehostID = detail.ID
		resp.RepoOwner = owner
		resp.RepoName = r
		resp.BranchName = branch
		resp.LoadPath = loadPath
		resp.LoadFromDir = true
		return resp, nil

	} else if resp.Source == setting.SourceFromGUI {
		yamls := strings.Split(resp.Yaml, "---")
		for _, y := range yamls {
			data, err := yaml.YAMLToJSON([]byte(y))
			if err != nil {
				log.Errorf("convert yaml to json failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			var result interface{}
			err = json.Unmarshal(data, &result)
			if err != nil {
				log.Errorf("unmarshal yaml data failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			yamlPreview := yamlPreview{}
			err = json.Unmarshal(data, &yamlPreview)
			if err != nil {
				log.Errorf("unmarshal yaml data failed, yaml:%s, err:%v", y, err)
				return nil, err
			}

			if resp.GUIConfig == nil {
				resp.GUIConfig = new(commonmodels.GUIConfig)
			}

			switch yamlPreview.Kind {
			case "Deployment":
				resp.GUIConfig.Deployment = result
			case "Ingress":
				resp.GUIConfig.Ingress = result
			case "Service":
				resp.GUIConfig.Service = result
			}
		}
	}

	return resp, nil
}

func UpdatePmServiceTemplate(username string, args *ServiceTmplBuildObject, log *zap.SugaredLogger) error {
	//该请求来自环境中的服务更新时，from=createEnv
	if args.ServiceTmplObject.From == "" {
		if err := UpdateBuild(username, args.Build, log); err != nil {
			return err
		}
	}

	//先比较healthcheck是否有变动
	preService, err := GetServiceTemplate(args.ServiceTmplObject.ServiceName, setting.PMDeployType, args.ServiceTmplObject.ProductName, setting.ProductStatusDeleting, args.ServiceTmplObject.Revision, log)
	if err != nil {
		return err
	}

	preBuildName := preService.BuildName

	//更新服务
	serviceTemplate := fmt.Sprintf(setting.ServiceTemplateCounterName, preService.ServiceName, preService.ProductName)
	rev, err := commonrepo.NewCounterColl().GetNextSeq(serviceTemplate)
	if err != nil {
		return err
	}
	preService.HealthChecks = args.ServiceTmplObject.HealthChecks
	preService.EnvConfigs = args.ServiceTmplObject.EnvConfigs
	preService.Revision = rev
	preService.CreateBy = username
	preService.BuildName = args.Build.Name

	if err := commonrepo.NewServiceColl().Delete(preService.ServiceName, setting.PMDeployType, args.ServiceTmplObject.ProductName, setting.ProductStatusDeleting, preService.Revision); err != nil {
		return err
	}

	if preBuildName != args.Build.Name {
		preBuild, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: preBuildName})
		if err != nil {
			return e.ErrUpdateService.AddDesc("get pre build failed")
		}

		var targets []*commonmodels.ServiceModuleTarget
		for _, serviceModule := range preBuild.Targets {
			if serviceModule.ServiceName != args.ServiceTmplObject.ServiceName {
				targets = append(targets, serviceModule)
			}
		}
		preBuild.Targets = targets

		if err = UpdateBuild(username, preBuild, log); err != nil {
			return e.ErrUpdateService.AddDesc("update pre build failed")
		}

		currentBuild, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: args.Build.Name})
		if err != nil {
			return e.ErrUpdateService.AddDesc("get current build failed")
		}
		var include bool
		for _, serviceModule := range currentBuild.Targets {
			if serviceModule.ServiceName == args.ServiceTmplObject.ServiceName {
				include = true
				break
			}
		}

		if !include {
			currentBuild.Targets = append(currentBuild.Targets, &commonmodels.ServiceModuleTarget{
				ProductName:   args.ServiceTmplObject.ProductName,
				ServiceName:   args.ServiceTmplObject.ServiceName,
				ServiceModule: args.ServiceTmplObject.ServiceName,
			})
			if err = UpdateBuild(username, currentBuild, log); err != nil {
				return e.ErrUpdateService.AddDesc("update current build failed")
			}
		}

	}

	if err := commonrepo.NewServiceColl().Create(preService); err != nil {
		return err
	}
	return nil
}

func DeleteServiceWebhookByName(serviceName, productName string, logger *zap.SugaredLogger) {
	svc, err := commonrepo.NewServiceColl().Find(&commonrepo.ServiceFindOption{ServiceName: serviceName, ProductName: productName})
	if err != nil {
		logger.Errorf("Failed to get service %s, error: %s", serviceName, err)
		return
	}
	ProcessServiceWebhook(nil, svc, serviceName, logger)
}

func ProcessServiceWebhook(updated, current *commonmodels.Service, serviceName string, logger *zap.SugaredLogger) {
	var action string
	var updatedHooks, currentHooks []*webhook.WebHook
	if updated != nil {
		if updated.Source == setting.SourceFromZadig || updated.Source == setting.SourceFromGerrit || updated.Source == "" || updated.Source == setting.SourceFromExternal {
			return
		}
		action = "add"
		address := getAddressFromPath(updated.SrcPath, updated.RepoOwner, updated.RepoName, logger.Desugar())
		if address == "" {
			return
		}
		updatedHooks = append(updatedHooks, &webhook.WebHook{Owner: updated.RepoOwner, Repo: updated.RepoName, Address: address, Name: "trigger", CodeHostID: updated.CodehostID})
	}
	if current != nil {
		if current.Source == setting.SourceFromZadig || current.Source == setting.SourceFromGerrit || current.Source == "" || current.Source == setting.SourceFromExternal {
			return
		}
		action = "remove"
		address := getAddressFromPath(current.SrcPath, current.RepoOwner, current.RepoName, logger.Desugar())
		if address == "" {
			return
		}
		currentHooks = append(currentHooks, &webhook.WebHook{Owner: current.RepoOwner, Repo: current.RepoName, Address: address, Name: "trigger", CodeHostID: current.CodehostID})
	}
	if updated != nil && current != nil {
		action = "update"
	}

	logger.Debugf("Start to %s webhook for service %s", action, serviceName)
	err := ProcessWebhook(updatedHooks, currentHooks, webhook.ServicePrefix+serviceName, logger)
	if err != nil {
		logger.Errorf("Failed to process WebHook, error: %s", err)
	}

}

func getAddressFromPath(path, owner, repo string, logger *zap.Logger) string {
	res := strings.Split(path, fmt.Sprintf("/%s/%s/", owner, repo))
	if len(res) != 2 {
		logger.With(zap.String("path", path), zap.String("owner", owner), zap.String("repo", repo)).DPanic("Invalid path")
		return ""
	}
	return res[0]
}

// get values from source flat map
// convert map[k]absolutePath  to  map[k]value
func getValuesByPath(paths map[string]string, flatMap map[string]interface{}) map[string]interface{} {
	ret := make(map[string]interface{})
	for k, path := range paths {
		if value, ok := flatMap[path]; ok {
			ret[k] = value
		} else {
			ret[k] = nil
		}
	}
	return ret
}

// GeneImageURI generate valid image uri, legal formats:
// {repo}
// {repo}/{image}
// {repo}/{image}:{tag}
// {repo}:{tag}
// {image}:{tag}
// {image}
func GeneImageURI(pathData map[string]string, flatMap map[string]interface{}) (string, error) {
	valuesMap := getValuesByPath(pathData, flatMap)
	ret := ""
	// if repo value is set, use as repo
	if repo, ok := valuesMap[setting.PathSearchComponentRepo]; ok {
		ret = fmt.Sprintf("%v", repo)
		ret = strings.TrimSuffix(ret, "/")
	}
	// if image value is set, append to repo, if repo is not set, image values represents repo+image
	if image, ok := valuesMap[setting.PathSearchComponentImage]; ok {
		imageStr := fmt.Sprintf("%v", image)
		if ret == "" {
			ret = imageStr
		} else {
			ret = fmt.Sprintf("%s/%s", ret, imageStr)
		}
	}
	if ret == "" {
		return "", errors.New("image name not found")
	}
	// if tag is set, append to current uri, if not set ignore
	if tag, ok := valuesMap[setting.PathSearchComponentTag]; ok {
		tagStr := fmt.Sprintf("%v", tag)
		if tagStr != "" {
			ret = fmt.Sprintf("%s:%s", ret, tagStr)
		}
	}
	return ret, nil
}

// ExtractImageName extract image name from total image uri
func ExtractImageName(imageURI string) string {
	subMatchAll := imageParseRegex.FindStringSubmatch(imageURI)
	exNames := imageParseRegex.SubexpNames()
	for i, matchedStr := range subMatchAll {
		if i != 0 && matchedStr != "" && matchedStr != ":" {
			if exNames[i] == "image" {
				return matchedStr
			}
		}
	}
	return ""
}

func parseImagesByPattern(nested map[string]interface{}, patterns []map[string]string) ([]*models.Container, error) {
	flatMap, err := converter.Flatten(nested)
	if err != nil {
		return nil, err
	}
	matchedPath, err := yamlutil.SearchByPattern(flatMap, patterns)
	if err != nil {
		return nil, err
	}
	ret := make([]*models.Container, 0)
	for _, searchResult := range matchedPath {
		imageUrl, err := GeneImageURI(searchResult, flatMap)
		if err != nil {
			return nil, err
		}
		ret = append(ret, &models.Container{
			Name:  ExtractImageName(imageUrl),
			Image: imageUrl,
			ImagePath: &models.ImagePathSpec{
				Repo:  searchResult[setting.PathSearchComponentRepo],
				Image: searchResult[setting.PathSearchComponentImage],
				Tag:   searchResult[setting.PathSearchComponentTag],
			},
		})
	}
	return ret, nil
}

func ParseImagesByRules(nested map[string]interface{}, matchRules []*template.ImageSearchingRule) ([]*models.Container, error) {
	patterns := make([]map[string]string, 0)
	for _, rule := range matchRules {
		if !rule.InUse {
			continue
		}
		patterns = append(patterns, rule.GetSearchingPattern())
	}
	return parseImagesByPattern(nested, patterns)
}

// get patterns used to parse images from yaml
func getServiceParsePatterns(productName string) ([]map[string]string, error) {
	productInfo, err := templaterepo.NewProductColl().Find(productName)
	if err != nil {
		return nil, err
	}
	ret := make([]map[string]string, 0)
	for _, rule := range productInfo.ImageSearchingRules {
		if !rule.InUse {
			continue
		}
		ret = append(ret, rule.GetSearchingPattern())
	}

	// rules are never edited, use preset rules
	if len(ret) == 0 {
		ret = presetPatterns
	}
	return ret, nil
}

// ParseImagesForProductService for product service
func ParseImagesForProductService(nested map[string]interface{}, serviceName, productName string) ([]*models.Container, error) {
	patterns, err := getServiceParsePatterns(productName)
	if err != nil {
		log.Errorf("failed to get image parse patterns for service %s in project %s, err: %s", serviceName, productName, err)
		return nil, errors.New("failed to get image parse patterns")
	}
	return parseImagesByPattern(nested, patterns)
}

// ParseImagesByPresetRules parse images from flat yaml map with preset rules
func ParseImagesByPresetRules(flatMap map[string]interface{}) ([]map[string]string, error) {
	return yamlutil.SearchByPattern(flatMap, presetPatterns)
}

func GetPresetRules() []*template.ImageSearchingRule {
	ret := make([]*template.ImageSearchingRule, 0, len(presetPatterns))
	for id, pattern := range presetPatterns {
		ret = append(ret, &template.ImageSearchingRule{
			Repo:     pattern[setting.PathSearchComponentRepo],
			Image:    pattern[setting.PathSearchComponentImage],
			Tag:      pattern[setting.PathSearchComponentTag],
			InUse:    true,
			PresetId: id + 1,
		})
	}
	return ret
}

type Variable struct {
	SERVICE        string
	TIMESTAMP      string
	TASK_ID        string
	REPO_COMMIT_ID string
	PROJECT        string
	ENV_NAME       string
	REPO_TAG       string
	REPO_BRANCH    string
	REPO_PR        string
}

func ReplaceRuleVariable(rule string, replaceValue *Variable) string {
	template, err := templ.New("replaceRuleVariable").Parse(rule)
	if err != nil {
		log.Errorf("replaceRuleVariable Parse err:%s", err)
		return rule
	}
	var replaceRuleVariable = templ.Must(template, err)
	payload := bytes.NewBufferString("")
	err = replaceRuleVariable.Execute(payload, replaceValue)
	if err != nil {
		log.Errorf("replaceRuleVariable Execute err:%s", err)
		return rule
	}

	return payload.String()
}
