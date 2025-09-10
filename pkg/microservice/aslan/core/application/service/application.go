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
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"

	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/pm"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	helmtool "github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/util"
)

// Validation helpers
func validateApplicationBaseFields(app *commonmodels.Application) error {
	if app.Name == "" || app.Key == "" || app.Project == "" {
		return e.ErrInvalidParam.AddDesc("name, key, project are required")
	}
	return nil
}

func validateAndPruneCustomFields(app *commonmodels.Application) error {
	defs, err := commonrepo.NewApplicationFieldDefinitionColl().List(context.Background())
	if err != nil {
		return err
	}
	defMap := map[string]*commonmodels.ApplicationFieldDefinition{}
	for _, d := range defs {
		defMap[d.Key] = d
	}

	values := map[string]interface{}{}
	if app.CustomFields != nil {
		values = app.CustomFields
	}

	for key, def := range defMap {
		if def.Source == config.ApplicationFieldSourceBuiltin {
			continue
		}
		
		if def.Required {
			v, ok := values[key]
			if !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("missing required custom field: %s", key))
			}
			if isEmptyByType(def.Type, v) {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("required custom field %s must not be empty", key))
			}
		}
	}

	for key, val := range values {
		def, ok := defMap[key]
		if !ok {
			// prune undefined fields from customFields
			if app.CustomFields != nil {
				delete(app.CustomFields, key)
			}
			continue
		}
		switch def.Type {
		case config.ApplicationCustomFieldTypeText, config.ApplicationCustomFieldTypeSingleSelect, config.ApplicationCustomFieldTypeLink, config.ApplicationCustomFieldTypeUser, config.ApplicationCustomFieldTypeUserGroup, config.ApplicationCustomFieldTypeProject:
			if _, ok := val.(string); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be string", key))
			}
			if def.Type == config.ApplicationCustomFieldTypeSingleSelect && len(def.Options) > 0 {
				s := val.(string)
				if !contains(def.Options, s) {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s not in options", key))
				}
			}
		case config.ApplicationCustomFieldTypeNumber, config.ApplicationCustomFieldTypeDatetime:
			if _, ok := val.(float64); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be number", key))
			}
		case config.ApplicationCustomFieldTypeMultiSelect:
			arr, ok := val.([]interface{})
			if !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be array of strings", key))
			}
			for _, it := range arr {
				s, ok := it.(string)
				if !ok {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be array of strings", key))
				}
				if len(def.Options) > 0 && !contains(def.Options, s) {
					return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s contains value not in options", key))
				}
			}
		case config.ApplicationCustomFieldTypeBool:
			if _, ok := val.(bool); !ok {
				return e.ErrInvalidParam.AddDesc(fmt.Sprintf("custom field %s must be bool", key))
			}

		default:
			return e.ErrInvalidParam.AddDesc(fmt.Sprintf("unsupported custom field type: %s", def.Type))
		}
	}

	return nil
}

// isEmptyByType determines emptiness for required field validation, per field definition type.
func isEmptyByType(defType config.ApplicationCustomFieldType, v interface{}) bool {
	switch defType {
	case config.ApplicationCustomFieldTypeText, config.ApplicationCustomFieldTypeSingleSelect, config.ApplicationCustomFieldTypeLink, config.ApplicationCustomFieldTypeUser, config.ApplicationCustomFieldTypeUserGroup, config.ApplicationCustomFieldTypeProject:
		s, ok := v.(string)
		return !ok || strings.TrimSpace(s) == ""
	case config.ApplicationCustomFieldTypeNumber, config.ApplicationCustomFieldTypeDatetime:
		switch v.(type) {
		case float64, int64:
			return false
		default:
			return true
		}
	case config.ApplicationCustomFieldTypeBool:
		_, ok := v.(bool)
		return !ok
	case config.ApplicationCustomFieldTypeMultiSelect:
		arr, ok := v.([]interface{})
		if !ok {
			return true
		}
		if len(arr) == 0 {
			return true
		}
		for _, it := range arr {
			s, ok := it.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return true
			}
		}
		return false
	default:
		return true
	}
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func CreateApplication(app *commonmodels.Application, logger *zap.SugaredLogger) (*commonmodels.Application, error) {
	if app == nil {
		return nil, e.ErrInvalidParam.AddDesc("empty body")
	}
	if err := validateApplicationBaseFields(app); err != nil {
		return nil, err
	}
	if err := validateAndPruneCustomFields(app); err != nil {
		return nil, err
	}
	oid, err := commonrepo.NewApplicationColl().Create(context.Background(), app)
	if err != nil {
		return nil, err
	}
	app.ID = oid
	return app, nil
}

func BulkCreateApplications(apps []*commonmodels.Application, logger *zap.SugaredLogger) error {
	if len(apps) == 0 {
		return nil
	}
	// validate first to fail-fast before transaction
	for _, app := range apps {
		if app == nil {
			return e.ErrInvalidParam.AddDesc("empty body in list")
		}
		if err := validateApplicationBaseFields(app); err != nil {
			return err
		}
		if err := validateAndPruneCustomFields(app); err != nil {
			return err
		}
	}

	_, err := commonrepo.NewApplicationColl().BulkCreate(context.TODO(), apps)
	if err != nil {
		return err
	}
	return nil
}

func GetApplication(id string, logger *zap.SugaredLogger) (*commonmodels.Application, error) {
	ctx := context.Background()
	app, err := commonrepo.NewApplicationColl().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	plugins := make([]string, 0)
	plist, err := commonrepo.NewPluginColl().List()
	if err != nil {
		logger.Warnf("failed to list plugins: %v", err)
		app.Plugins = plugins
		return app, nil
	}

	defs, err := commonrepo.NewApplicationFieldDefinitionColl().List(ctx)
	if err != nil {
		logger.Warnf("failed to list application field definitions: %v", err)
		defs = nil
	}
	defMap := map[string]*commonmodels.ApplicationFieldDefinition{}
	for _, d := range defs {
		defMap[d.Key] = d
	}

	for _, p := range plist {
		if p == nil || !p.Enabled || strings.ToLower(p.Type) != "tab" {
			continue
		}

		if len(p.Filters) == 0 {
			plugins = append(plugins, p.ID.Hex())
			continue
		}

		if appMatchesFilters(app, p.Filters, defMap) {
			plugins = append(plugins, p.ID.Hex())
		}
	}
	app.Plugins = plugins
	return app, nil
}

func appMatchesFilters(app *commonmodels.Application, filters []*commonmodels.PluginFilter, defs map[string]*commonmodels.ApplicationFieldDefinition) bool {
	for _, f := range filters {
		path, fType, err := resolveField(f.Field, defs)
		if err != nil {
			return false
		}
		if !matchFilterOnApp(app, path, fType, f) {
			return false
		}
	}
	return true
}

func matchFilterOnApp(app *commonmodels.Application, path, fType string, f *commonmodels.PluginFilter) bool {
	verb := strings.ToLower(f.Verb)

	val, ok := getAppFieldValue(app, path)
	if !ok {
		return false
	}

	switch fType {
	case string(config.ApplicationFilterFieldTypeNumber):
		fv, err := toFloat64(f.Value)
		if err != nil {
			return false
		}
		var cur float64
		switch t := val.(type) {
		case float64:
			cur = t
		case int64:
			cur = float64(t)
		case int:
			cur = float64(t)
		case json.Number:
			cur, err = t.Float64()
			if err != nil {
				return false
			}
		case string:
			cur, err = strconv.ParseFloat(t, 64)
			if err != nil {
				return false
			}
		default:
			return false
		}
		switch verb {
		case string(config.ApplicationFilterActionEq):
			return cur == fv
		case string(config.ApplicationFilterActionNe):
			return cur != fv
		case string(config.ApplicationFilterActionLt):
			return cur < fv
		case string(config.ApplicationFilterActionLte):
			return cur <= fv
		case string(config.ApplicationFilterActionGt):
			return cur > fv
		case string(config.ApplicationFilterActionGte):
			return cur >= fv
		default:
			return false
		}

	case string(config.ApplicationFilterFieldTypeBool):
		b, ok := f.Value.(bool)
		if !ok {
			return false
		}
		cur, ok := val.(bool)
		if !ok {
			return false
		}
		// only supports IS
		return verb == string(config.ApplicationFilterActionIs) && cur == b

	case string(config.ApplicationFilterFieldTypeArray):
		arr := toStringArray(val)
		if arr == nil {
			return false
		}
		switch verb {
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return false
			}
			return containsAny(arr, vals)
		case string(config.ApplicationFilterActionContains):
			s, err := toString(f.Value)
			if err != nil {
				return false
			}
			for _, it := range arr {
				if it == s {
					return true
				}
			}
			return false
		case string(config.ApplicationFilterActionNotContains):
			s, err := toString(f.Value)
			if err != nil {
				return false
			}
			for _, it := range arr {
				if it == s {
					return false
				}
			}
			return true
		case string(config.ApplicationFilterActionIsEmpty):
			return len(arr) == 0
		case string(config.ApplicationFilterActionIsNotEmpty):
			return len(arr) > 0
		default:
			return false
		}

	case string(config.ApplicationFilterFieldTypeString):
		s, err := toString(f.Value)
		if err != nil {
			return false
		}
		cur, ok := val.(string)
		if !ok {
			return false
		}
		// plugin filters: default case-insensitive
		s = strings.ToLower(s)
		cur = strings.ToLower(cur)
		switch verb {
		case string(config.ApplicationFilterActionEq):
			return cur == s
		case string(config.ApplicationFilterActionNe):
			return cur != s
		case string(config.ApplicationFilterActionBeginsWith):
			return strings.HasPrefix(cur, s)
		case string(config.ApplicationFilterActionNotBeginsWith):
			return !strings.HasPrefix(cur, s)
		case string(config.ApplicationFilterActionEndsWith):
			return strings.HasSuffix(cur, s)
		case string(config.ApplicationFilterActionNotEndsWith):
			return !strings.HasSuffix(cur, s)
		case string(config.ApplicationFilterActionContains):
			return strings.Contains(cur, s)
		case string(config.ApplicationFilterActionNotContains):
			return !strings.Contains(cur, s)
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return false
			}
			for i := range vals {
				vals[i] = strings.ToLower(vals[i])
			}
			for _, v := range vals {
				if cur == v {
					return true
				}
			}
			return false
		default:
			return false
		}
	default:
		return false
	}
}

func getAppFieldValue(app *commonmodels.Application, path string) (interface{}, bool) {
	switch path {
	case "name":
		return app.Name, true
	case "key":
		return app.Key, true
	case "project":
		return app.Project, true
	case "type":
		return app.Type, true
	case "owner":
		return app.Owner, true
	case "description":
		return app.Description, true
	case "create_time":
		return app.CreateTime, true
	case "update_time":
		return app.UpdateTime, true
	case "repository.codehost_id":
		if app.Repository == nil {
			return nil, false
		}
		return app.Repository.CodehostID, true
	default:
		if strings.HasPrefix(path, "custom_fields.") {
			key := strings.TrimPrefix(path, "custom_fields.")
			if app.CustomFields == nil {
				return nil, false
			}
			v, ok := app.CustomFields[key]
			return v, ok
		}
		return nil, false
	}
}

func toStringArray(v interface{}) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []interface{}:
		res := make([]string, 0, len(t))
		for _, it := range t {
			if s, ok := it.(string); ok {
				res = append(res, s)
			}
		}
		return res
	default:
		return nil
	}
}

func containsAny(have []string, want []string) bool {
	set := make(map[string]struct{}, len(have))
	for _, s := range have {
		set[s] = struct{}{}
	}
	for _, s := range want {
		if _, ok := set[s]; ok {
			return true
		}
	}
	return false
}

func UpdateApplication(id string, app *commonmodels.Application, logger *zap.SugaredLogger) error {
	if app == nil {
		return e.ErrInvalidParam.AddDesc("empty body")
	}
	old, err := commonrepo.NewApplicationColl().GetByID(context.Background(), id)
	if err != nil {
		return err
	}

	if app.Key != old.Key {
		return e.ErrInvalidParam.AddDesc("key is immutable")
	}

	if err := validateApplicationBaseFields(app); err != nil {
		return err
	}
	if err := validateAndPruneCustomFields(app); err != nil {
		return err
	}

	app.ID = old.ID
	if err := commonrepo.NewApplicationColl().UpdateByID(context.Background(), id, app); err != nil {
		return err
	}
	return nil
}

func DeleteApplication(id string, logger *zap.SugaredLogger) error {
	return commonrepo.NewApplicationColl().DeleteByID(context.Background(), id)
}

// Search with filter list model (re-used from earlier design), with validation unaffected.
type Filter struct {
	Field           string      `json:"field"`
	Verb            string      `json:"verb"`
	Value           interface{} `json:"value"`
	CaseInsensitive *bool       `json:"case_insensitive,omitempty"`
	ExcludeNulls    *bool       `json:"exclude_nulls,omitempty"`
}

type SearchApplicationsRequest struct {
	Page            int64     `json:"page"`
	PageSize        int64     `json:"page_size"`
	Query           string    `json:"query"`
	Filters         []*Filter `json:"filters"`
	SortBy          string    `json:"sort_by"`
	SortOrder       string    `json:"sort_order"`
	SortInsensitive bool      `json:"sort_insensitive"`
}

func SearchApplications(req *SearchApplicationsRequest, logger *zap.SugaredLogger) ([]*commonmodels.Application, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	defs, _ := commonrepo.NewApplicationFieldDefinitionColl().List(context.Background())
	defMap := map[string]*commonmodels.ApplicationFieldDefinition{}
	for _, d := range defs {
		defMap[d.Key] = d
	}
	query := bson.M{}
	ands := make([]bson.M, 0)
	if strings.TrimSpace(req.Query) != "" {
		ands = append(ands, bson.M{"$or": []bson.M{{"name": bson.M{"$regex": req.Query, "$options": "i"}}, {"key": bson.M{"$regex": req.Query, "$options": "i"}}}})
	}
	if len(req.Filters) > 0 {
		exprs, err := buildFilterQuery(req.Filters, defMap)
		if err != nil {
			return nil, 0, err
		}
		ands = append(ands, exprs...)
	}
	if len(ands) > 0 {
		query["$and"] = ands
	}
	order := int32(1)
	if strings.ToLower(req.SortOrder) == "desc" {
		order = -1
	}
	sortBy := req.SortBy
	if sortBy == "" {
		sortBy = "update_time"
	}
	sort := bson.D{{Key: sortBy, Value: order}}
	list, total, err := commonrepo.NewApplicationColl().List(context.Background(), &commonrepo.ApplicationListOptions{Query: query, Sort: sort, Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

type GetBizDirServiceDetailResponse struct {
	ProjectName  string   `json:"project_name"`
	EnvName      string   `json:"env_name"`
	EnvAlias     string   `json:"env_alias"`
	Production   bool     `json:"production"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Status       string   `json:"status"`
	Images       []string `json:"images"`
	ChartVersion string   `json:"chart_version"`
	UpdateTime   int64    `json:"update_time"`
	Error        string   `json:"error"`
}

func ListApplicationEnvs(id string, logger *zap.SugaredLogger) ([]*GetBizDirServiceDetailResponse, error) {
	resp := make([]*GetBizDirServiceDetailResponse, 0)

	app, err := commonrepo.NewApplicationColl().GetByID(context.Background(), id)
	if err != nil {
		return nil, err
	}

	if app.Project == "" {
		return nil, fmt.Errorf("project is required to find envs")
	}

	project, err := templaterepo.NewProductColl().Find(app.Project)
	if err != nil {
		return nil, fmt.Errorf("failed to find project %s, error: %v", app.Project, err)
	}

	if app.TestingServiceName != "" {
		envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       app.Project,
			Production: util.GetBoolPointer(false),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list product %s, error: %v", app.Project, err)
		}

		for _, env := range envs {
			prodSvc := env.GetServiceMap()[app.TestingServiceName]
			if prodSvc == nil {
				// not deployed in this env
				continue
			}

			if project.IsK8sYamlProduct() || project.IsHostProduct() {
				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        app.TestingServiceName,
					Type:        setting.K8SDeployType,
					UpdateTime:  prodSvc.UpdateTime,
				}
				serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
					ServiceName: prodSvc.ServiceName,
					Revision:    prodSvc.Revision,
					ProductName: prodSvc.ProductName,
				}, env.Production)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					resp = append(resp, detail)
					continue
				}

				cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service status & image info due to kube client creation, err: %s", err)
					resp = append(resp, detail)
					continue
				}
				inf, err := clientmanager.NewKubeClientManager().GetInformer(env.ClusterID, env.Namespace)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service status & image info due to kube informer creation, err: %s", err)
					resp = append(resp, detail)
					continue
				}

				serviceStatus := commonservice.QueryPodsStatus(env, serviceTmpl, app.TestingServiceName, cls, inf, log.SugaredLogger())
				detail.Status = serviceStatus.PodStatus
				detail.Images = serviceStatus.Images

				resp = append(resp, detail)
			} else if project.IsHelmProduct() {
				svcToReleaseNameMap, err := commonutil.GetServiceNameToReleaseNameMap(env)
				if err != nil {
					return nil, fmt.Errorf("failed to build release-service map: %s", err)
				}
				releaseName := svcToReleaseNameMap[app.TestingServiceName]
				if releaseName == "" {
					return nil, fmt.Errorf("release name not found for service %s", app.TestingServiceName)
				}

				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        fmt.Sprintf("%s(%s)", releaseName, app.TestingServiceName),
					Type:        setting.HelmDeployType,
				}

				helmClient, err := helmtool.NewClientFromNamespace(env.ClusterID, env.Namespace)
				if err != nil {
					log.Errorf("[%s][%s] NewClientFromRestConf error: %s", env.EnvName, app.Project, err)
					return nil, fmt.Errorf("failed to init helm client, err: %s", err)
				}

				listClient := action.NewList(helmClient.ActionConfig)
				listClient.Filter = releaseName
				releases, err := listClient.Run()
				if err != nil {
					return nil, e.ErrGetBizDirServiceDetail.AddErr(fmt.Errorf("failed to list helm releases by %s, error: %v", app.TestingServiceName, err))
				}
				if len(releases) == 0 {
					resp = append(resp, detail)
					continue
				}
				if len(releases) > 1 {
					detail.Error = "helm release number is not equal to 1"
					log.Warnf("helm release number is not equal to 1")
					resp = append(resp, detail)
					continue
				}

				detail.Status = string(releases[0].Info.Status)
				detail.ChartVersion = releases[0].Chart.Metadata.Version
				detail.UpdateTime = releases[0].Info.LastDeployed.Unix()

				resp = append(resp, detail)
			} else if project.IsCVMProduct() {
				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        app.TestingServiceName,
					Type:        project.ProductFeature.DeployType,
					UpdateTime:  prodSvc.UpdateTime,
				}

				serviceTmpl, err := commonservice.GetServiceTemplate(
					prodSvc.ServiceName, setting.PMDeployType, prodSvc.ProductName, "", prodSvc.Revision, false, log.SugaredLogger(),
				)
				if err != nil {
					detail.Error = fmt.Sprintf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					log.Warnf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					resp = append(resp, detail)
					continue
				}

				if len(serviceTmpl.EnvStatuses) > 0 {
					envStatuses := make([]*commonmodels.EnvStatus, 0)
					filterEnvStatuses, err := pm.GenerateEnvStatus(serviceTmpl.EnvConfigs, log.NopSugaredLogger())
					if err != nil {
						detail.Error = fmt.Sprintf("failed to generate env status for productName: %s, serviceName: %s, revision %d, error: %v", prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
						log.Warnf("failed to generate env status for productName: %s, serviceName: %s, revision %d, error: %v", prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
						resp = append(resp, detail)
						continue
					}
					filterEnvStatusSet := sets.NewString()
					for _, v := range filterEnvStatuses {
						filterEnvStatusSet.Insert(v.Address)
					}
					for _, envStatus := range serviceTmpl.EnvStatuses {
						if envStatus.EnvName == env.EnvName && filterEnvStatusSet.Has(envStatus.Address) {
							envStatuses = append(envStatuses, envStatus)
						}
					}

					if len(envStatuses) > 0 {
						total := 0
						running := 0
						for _, envStatus := range envStatuses {
							total++
							if envStatus.Status == setting.PodRunning {
								running++
							}
						}
						detail.Status = fmt.Sprintf("%d/%d", running, total)
					}
				}

				resp = append(resp, detail)
			}
		}
	}

	if app.ProductionServiceName != "" {
		envs, err := commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
			Name:       app.Project,
			Production: util.GetBoolPointer(true),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list product %s, error: %v", app.Project, err)
		}

		for _, env := range envs {
			prodSvc := env.GetServiceMap()[app.ProductionServiceName]
			if prodSvc == nil {
				// not deployed in this env
				continue
			}

			if project.IsK8sYamlProduct() || project.IsHostProduct() {
				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        app.ProductionServiceName,
					Type:        setting.K8SDeployType,
					UpdateTime:  prodSvc.UpdateTime,
				}
				serviceTmpl, err := repository.QueryTemplateService(&commonrepo.ServiceFindOption{
					ServiceName: prodSvc.ServiceName,
					Revision:    prodSvc.Revision,
					ProductName: prodSvc.ProductName,
				}, env.Production)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					resp = append(resp, detail)
					continue
				}

				cls, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(env.ClusterID)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service status & image info due to kube client creation, err: %s", err)
					resp = append(resp, detail)
					continue
				}
				inf, err := clientmanager.NewKubeClientManager().GetInformer(env.ClusterID, env.Namespace)
				if err != nil {
					detail.Error = err.Error()
					log.Warnf("failed to get service status & image info due to kube informer creation, err: %s", err)
					resp = append(resp, detail)
					continue
				}

				serviceStatus := commonservice.QueryPodsStatus(env, serviceTmpl, app.ProductionServiceName, cls, inf, log.SugaredLogger())
				detail.Status = serviceStatus.PodStatus
				detail.Images = serviceStatus.Images

				resp = append(resp, detail)
			} else if project.IsHelmProduct() {
				svcToReleaseNameMap, err := commonutil.GetServiceNameToReleaseNameMap(env)
				if err != nil {
					return nil, fmt.Errorf("failed to build release-service map: %s", err)
				}
				releaseName := svcToReleaseNameMap[app.ProductionServiceName]
				if releaseName == "" {
					return nil, fmt.Errorf("release name not found for service %s", app.ProductionServiceName)
				}

				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        fmt.Sprintf("%s(%s)", releaseName, app.TestingServiceName),
					Type:        setting.HelmDeployType,
				}

				helmClient, err := helmtool.NewClientFromNamespace(env.ClusterID, env.Namespace)
				if err != nil {
					log.Errorf("[%s][%s] NewClientFromRestConf error: %s", env.EnvName, app.Project, err)
					return nil, fmt.Errorf("failed to init helm client, err: %s", err)
				}

				listClient := action.NewList(helmClient.ActionConfig)
				listClient.Filter = releaseName
				releases, err := listClient.Run()
				if err != nil {
					return nil, e.ErrGetBizDirServiceDetail.AddErr(fmt.Errorf("failed to list helm releases by %s, error: %v", app.ProductionServiceName, err))
				}
				if len(releases) == 0 {
					resp = append(resp, detail)
					continue
				}
				if len(releases) > 1 {
					detail.Error = "helm release number is not equal to 1"
					log.Warnf("helm release number is not equal to 1")
					resp = append(resp, detail)
					continue
				}

				detail.Status = string(releases[0].Info.Status)
				detail.ChartVersion = releases[0].Chart.Metadata.Version
				detail.UpdateTime = releases[0].Info.LastDeployed.Unix()

				resp = append(resp, detail)
			} else if project.IsCVMProduct() {
				detail := &GetBizDirServiceDetailResponse{
					ProjectName: env.ProductName,
					EnvName:     env.EnvName,
					EnvAlias:    env.Alias,
					Production:  env.Production,
					Name:        app.ProductionServiceName,
					Type:        project.ProductFeature.DeployType,
					UpdateTime:  prodSvc.UpdateTime,
				}

				serviceTmpl, err := commonservice.GetServiceTemplate(
					prodSvc.ServiceName, setting.PMDeployType, prodSvc.ProductName, "", prodSvc.Revision, false, log.SugaredLogger(),
				)
				if err != nil {
					detail.Error = fmt.Sprintf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					log.Warnf("failed to get service template for productName: %s, serviceName: %s, revision %d, error: %v",
						prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
					resp = append(resp, detail)
					continue
				}

				if len(serviceTmpl.EnvStatuses) > 0 {
					envStatuses := make([]*commonmodels.EnvStatus, 0)
					filterEnvStatuses, err := pm.GenerateEnvStatus(serviceTmpl.EnvConfigs, log.NopSugaredLogger())
					if err != nil {
						detail.Error = fmt.Sprintf("failed to generate env status for productName: %s, serviceName: %s, revision %d, error: %v", prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
						log.Warnf("failed to generate env status for productName: %s, serviceName: %s, revision %d, error: %v", prodSvc.ProductName, prodSvc.ServiceName, prodSvc.Revision, err)
						resp = append(resp, detail)
						continue
					}
					filterEnvStatusSet := sets.NewString()
					for _, v := range filterEnvStatuses {
						filterEnvStatusSet.Insert(v.Address)
					}
					for _, envStatus := range serviceTmpl.EnvStatuses {
						if envStatus.EnvName == env.EnvName && filterEnvStatusSet.Has(envStatus.Address) {
							envStatuses = append(envStatuses, envStatus)
						}
					}

					if len(envStatuses) > 0 {
						total := 0
						running := 0
						for _, envStatus := range envStatuses {
							total++
							if envStatus.Status == setting.PodRunning {
								running++
							}
						}
						detail.Status = fmt.Sprintf("%d/%d", running, total)
					}
				}

				resp = append(resp, detail)
			}
		}
	}

	return resp, nil
}

func buildFilterQuery(filters []*Filter, defs map[string]*commonmodels.ApplicationFieldDefinition) ([]bson.M, error) {
	out := make([]bson.M, 0, len(filters))
	for _, f := range filters {
		path, fType, err := resolveField(f.Field, defs)
		if err != nil {
			return nil, err
		}
		expr, err := filterToExpr(path, fType, f)
		if err != nil {
			return nil, err
		}
		out = append(out, expr)
	}
	return out, nil
}

func resolveField(field string, defs map[string]*commonmodels.ApplicationFieldDefinition) (string, string, error) {
	if strings.HasPrefix(field, "custom_fields.") {
		key := strings.TrimPrefix(field, "custom_fields.")
		def, ok := defs[key]
		if !ok {
			return "", "", e.ErrInvalidParam.AddDesc("unknown custom field: " + key)
		}
		var cat string
		switch def.Type {
		case config.ApplicationCustomFieldTypeText, config.ApplicationCustomFieldTypeSingleSelect, config.ApplicationCustomFieldTypeLink, config.ApplicationCustomFieldTypeUser, config.ApplicationCustomFieldTypeUserGroup, config.ApplicationCustomFieldTypeProject:
			cat = string(config.ApplicationFilterFieldTypeString)
		case config.ApplicationCustomFieldTypeNumber, config.ApplicationCustomFieldTypeDatetime:
			cat = string(config.ApplicationFilterFieldTypeNumber)
		case config.ApplicationCustomFieldTypeBool:
			cat = string(config.ApplicationFilterFieldTypeBool)
		case config.ApplicationCustomFieldTypeMultiSelect:
			cat = string(config.ApplicationFilterFieldTypeArray)
		default:
			return "", "", e.ErrInvalidParam.AddDesc("unsupported custom field type: " + string(def.Type))
		}
		return field, cat, nil
	}
	switch field {
	case "name", "key", "project", "description", "testing_service_id", "production_service_id", "owner", "type":
		return field, string(config.ApplicationFilterFieldTypeString), nil
	case "repository.codehost_id", "create_time", "update_time":
		return field, string(config.ApplicationFilterFieldTypeNumber), nil
	default:
		return "", "", e.ErrInvalidParam.AddDesc("unknown field: " + field)
	}
}

func filterToExpr(path, fType string, f *Filter) (bson.M, error) {
	verb := strings.ToLower(f.Verb)
	ci := true
	if f.CaseInsensitive != nil {
		ci = *f.CaseInsensitive
	}
	excludeNulls := true
	if f.ExcludeNulls != nil {
		excludeNulls = *f.ExcludeNulls
	}
	wrapNeg := func(cond bson.M) bson.M {
		if excludeNulls {
			return bson.M{"$and": []bson.M{{path: bson.M{"$ne": nil}}, {path: cond}}}
		}
		return bson.M{path: cond}
	}
	switch fType {
	case string(config.ApplicationFilterFieldTypeNumber):
		switch verb {
		case string(config.ApplicationFilterActionEq):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: v}, nil
		case string(config.ApplicationFilterActionNe):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$ne": v}}, nil
		case string(config.ApplicationFilterActionLt):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$lt": v}}, nil
		case string(config.ApplicationFilterActionLte):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$lte": v}}, nil
		case string(config.ApplicationFilterActionGt):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$gt": v}}, nil
		case string(config.ApplicationFilterActionGte):
			v, err := toFloat64(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$gte": v}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported number verb: " + verb)
		}
	case string(config.ApplicationFilterFieldTypeArray):
		switch verb {
		case string(config.ApplicationFilterActionContains):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			// match if the array contains the element equal to s (MongoDB allows equality to match array elements)
			return bson.M{path: s}, nil
		case string(config.ApplicationFilterActionNotContains):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			// exclude arrays that contain the element equal to s; respect excludeNulls via wrapNeg
			return wrapNeg(bson.M{"$nin": bson.A{s}}), nil
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$in": vals}}, nil
		case string(config.ApplicationFilterActionIsEmpty):
			return bson.M{path: bson.M{"$size": 0}}, nil
		case string(config.ApplicationFilterActionIsNotEmpty):
			// use exists and not equal to empty array to represent non-empty array
			return bson.M{path: bson.M{"$exists": true, "$ne": bson.A{}}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported array verb: " + verb)
		}
	case string(config.ApplicationFilterFieldTypeBool):
		if verb != string(config.ApplicationFilterActionIs) {
			return nil, e.ErrInvalidParam.AddDesc("unsupported bool verb: " + verb)
		}
		b, ok := f.Value.(bool)
		if !ok {
			return nil, e.ErrInvalidParam.AddDesc("bool filter expects boolean value")
		}
		return bson.M{path: b}, nil
	case string(config.ApplicationFilterFieldTypeString):
		switch verb {
		case string(config.ApplicationFilterActionEq):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				return bson.M{path: bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(s)), "$options": "i"}}, nil
			}
			return bson.M{path: s}, nil
		case string(config.ApplicationFilterActionNe):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				return wrapNeg(bson.M{"$not": bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(s)), "$options": "i"}}), nil
			}
			return bson.M{path: bson.M{"$ne": s}}, nil
		case string(config.ApplicationFilterActionBeginsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$regex": "^" + escapeRegex(s), "$options": ciOpt(ci)}}, nil
		case string(config.ApplicationFilterActionNotBeginsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$regex": "^" + escapeRegex(s), "$options": ciOpt(ci)}}), nil
		case string(config.ApplicationFilterActionEndsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return bson.M{path: bson.M{"$regex": escapeRegex(s) + "$", "$options": ciOpt(ci)}}, nil
		case string(config.ApplicationFilterActionNotEndsWith):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$regex": escapeRegex(s) + "$", "$options": ciOpt(ci)}}), nil
		case string(config.ApplicationFilterActionContains):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			// substring match
			return bson.M{path: bson.M{"$regex": escapeRegex(s), "$options": ciOpt(ci)}}, nil
		case string(config.ApplicationFilterActionNotContains):
			s, err := toString(f.Value)
			if err != nil {
				return nil, err
			}
			return wrapNeg(bson.M{"$not": bson.M{"$regex": escapeRegex(s), "$options": ciOpt(ci)}}), nil
		case string(config.ApplicationFilterActionHasAnyOf):
			vals, err := toStringSlice(f.Value)
			if err != nil {
				return nil, err
			}
			if ci {
				ors := make([]bson.M, 0, len(vals))
				for _, v := range vals {
					ors = append(ors, bson.M{path: bson.M{"$regex": fmt.Sprintf("^%s$", escapeRegex(v)), "$options": "i"}})
				}
				return bson.M{"$or": ors}, nil
			}
			return bson.M{path: bson.M{"$in": vals}}, nil
		default:
			return nil, e.ErrInvalidParam.AddDesc("unsupported string verb: " + verb)
		}
	default:
		return nil, e.ErrInvalidParam.AddDesc("unsupported field type: " + fType)
	}
}

func escapeRegex(in string) string {
	specials := []string{"\\", ".", "*", "+", "?", "|", "(", ")", "[", "]", "{", "}", "^", "$"}
	for _, s := range specials {
		in = strings.ReplaceAll(in, s, "\\"+s)
	}
	return in
}

func ciOpt(ci bool) string {
	if ci {
		return "i"
	}
	return ""
}

func toFloat64(v interface{}) (float64, error) {
	switch t := v.(type) {
	case float64:
		return t, nil
	case int:
		return float64(t), nil
	case int64:
		return float64(t), nil
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0, e.ErrInvalidParam.AddErr(err)
		}
		return f, nil
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0, e.ErrInvalidParam.AddErr(err)
		}
		return f, nil
	default:
		return 0, e.ErrInvalidParam.AddDesc("invalid number")
	}
}

func toString(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	default:
		return "", e.ErrInvalidParam.AddDesc("invalid string")
	}
}

func toStringSlice(v interface{}) ([]string, error) {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, it := range t {
			s, err := toString(it)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	case []string:
		return t, nil
	default:
		return nil, e.ErrInvalidParam.AddDesc("invalid string array")
	}
}
