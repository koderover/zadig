/*
Copyright 2026 The KodeRover Authors.

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

package migrate

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	ua999ProjectPrefix    = "glb-"
	ua999BuildTemplate    = "qa-product-glb-java-spring"
	ua999OtelParamsKey    = "OTEL_SETTINGS_OTEL_PARAMS"
	ua999LurkerVersionKey = "LURKER_VERSION"
	ua999OtelParamsValue  = "{{.parameter.qa-product-glb.otel_params}}"
	ua999LurkerVersionVal = "{{.parameter.qa-product-glb.lurker_version}}"
)

func init() {
	upgradepath.RegisterHandler("4.2.0", "9.9.9", V420ToV999)
}

// V420ToV999 updates specific build variables in both build definitions and workflow build job variable configs.
func V420ToV999() error {
	buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{Name: ua999BuildTemplate})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			log.Warnf("build template %s not found, skip migration", ua999BuildTemplate)
			return nil
		}
		return fmt.Errorf("failed to find build template %s, err: %s", ua999BuildTemplate, err)
	}

	projects, err := templaterepo.NewProductColl().List()
	if err != nil {
		return fmt.Errorf("failed to list projects, err: %s", err)
	}

	buildUpdated := 0
	buildKVUpdated := 0
	workflowUpdated := 0
	workflowKVUpdated := 0

	for _, project := range projects {
		if project == nil || !strings.HasPrefix(project.ProductName, ua999ProjectPrefix) {
			continue
		}

		targetBuilds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{
			ProductName: project.ProductName,
			TemplateID:  buildTemplate.ID.Hex(),
		})
		if err != nil {
			return fmt.Errorf("failed to list builds in project %s, err: %s", project.ProductName, err)
		}
		if len(targetBuilds) == 0 {
			continue
		}

		buildNameSet := make(map[string]struct{}, len(targetBuilds))
		for _, build := range targetBuilds {
			if build == nil {
				continue
			}

			buildNameSet[build.Name] = struct{}{}

			changed, changedCount := update999BuildVariables(build)
			if !changed {
				continue
			}

			if err := commonrepo.NewBuildColl().Update(build); err != nil {
				return fmt.Errorf("failed to update build %s in project %s, err: %s", build.Name, project.ProductName, err)
			}
			buildUpdated++
			buildKVUpdated += changedCount
		}

		workflowCursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{
			ProjectName: project.ProductName,
		})
		if err != nil {
			return fmt.Errorf("failed to list workflows in project %s, err: %s", project.ProductName, err)
		}

		for workflowCursor.Next(context.Background()) {
			workflow := new(commonmodels.WorkflowV4)
			if err := workflowCursor.Decode(workflow); err != nil {
				log.Errorf("failed to decode workflow in project %s, err: %s", project.ProductName, err)
				continue
			}

			changed, changedCount := update999WorkflowBuildJobVariables(workflow, buildNameSet)
			if !changed {
				continue
			}

			if err := commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), workflow); err != nil {
				log.Warnf("failed to update workflow %s in project %s, err: %s", workflow.Name, workflow.Project, err)
				continue
			}

			workflowUpdated++
			workflowKVUpdated += changedCount
		}

		if err := workflowCursor.Err(); err != nil {
			return fmt.Errorf("failed to iterate workflows in project %s, err: %s", project.ProductName, err)
		}
		_ = workflowCursor.Close(context.Background())
	}

	log.Infof("ua999 done: build updated=%d, build kv updated=%d, workflow updated=%d, workflow kv updated=%d",
		buildUpdated, buildKVUpdated, workflowUpdated, workflowKVUpdated)
	return nil
}

func update999BuildVariables(build *commonmodels.Build) (bool, int) {
	if build == nil {
		return false, 0
	}

	totalChanged := 0
	if build.PreBuild != nil {
		totalChanged += update999BuildKeyVals(build.PreBuild.Envs)
	}

	for _, target := range build.Targets {
		if target == nil {
			continue
		}
		totalChanged += update999BuildKeyVals(target.Envs)
	}

	return totalChanged > 0, totalChanged
}

func update999WorkflowBuildJobVariables(workflow *commonmodels.WorkflowV4, buildNameSet map[string]struct{}) (bool, int) {
	if workflow == nil {
		return false, 0
	}

	workflowChanged := false
	totalChanged := 0

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job == nil || job.JobType != config.JobZadigBuild {
				continue
			}

			spec := new(commonmodels.ZadigBuildJobSpec)
			if err := commonmodels.IToi(job.Spec, spec); err != nil {
				log.Errorf("failed to decode build job %s in workflow %s/%s, err: %s", job.Name, workflow.Project, workflow.Name, err)
				continue
			}

			changed, changedCount := update999BuildJobSpec(spec, buildNameSet)
			if !changed {
				continue
			}

			job.Spec = spec
			workflowChanged = true
			totalChanged += changedCount
		}
	}

	return workflowChanged, totalChanged
}

func update999BuildJobSpec(spec *commonmodels.ZadigBuildJobSpec, buildNameSet map[string]struct{}) (bool, int) {
	if spec == nil {
		return false, 0
	}

	totalChanged := 0

	changedCount := update999ServiceBuildConfigVariables(spec.ServiceAndBuildsOptions, buildNameSet)
	totalChanged += changedCount

	return totalChanged > 0, totalChanged
}

func update999ServiceBuildConfigVariables(serviceAndBuilds []*commonmodels.ServiceAndBuild, buildNameSet map[string]struct{}) int {
	changed := 0

	for _, serviceAndBuild := range serviceAndBuilds {
		if serviceAndBuild == nil {
			continue
		}
		if _, ok := buildNameSet[serviceAndBuild.BuildName]; !ok {
			continue
		}
		changed += update999RuntimeKeyVals(serviceAndBuild.KeyVals)
	}

	return changed
}

func update999BuildKeyVals(kvs commonmodels.KeyValList) int {
	changed := 0
	for _, kv := range kvs {
		if kv == nil {
			continue
		}
		targetValue, ok := get999TargetValue(kv.Key)
		if !ok || kv.Value == targetValue {
			continue
		}

		kv.Value = targetValue
		changed++
	}
	return changed
}

func update999RuntimeKeyVals(kvs commonmodels.RuntimeKeyValList) int {
	changed := 0
	for _, kv := range kvs {
		if kv == nil || kv.KeyVal == nil {
			continue
		}

		targetValue, ok := get999TargetValue(kv.Key)
		if !ok {
			continue
		}

		updated := false
		if kv.Value != targetValue {
			kv.Value = targetValue
			updated = true
		}
		if kv.Source != config.ParamSourceReference {
			kv.Source = config.ParamSourceReference
			updated = true
		}

		if updated {
			changed++
		}
	}
	return changed
}

func get999TargetValue(key string) (string, bool) {
	switch key {
	case ua999OtelParamsKey:
		return ua999OtelParamsValue, true
	case ua999LurkerVersionKey:
		return ua999LurkerVersionVal, true
	default:
		return "", false
	}
}
