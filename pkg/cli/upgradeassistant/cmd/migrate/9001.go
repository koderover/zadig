/*
 * Copyright 2025 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"fmt"

	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
)

func init() {
	upgradepath.RegisterHandler("99.0.0", "99.0.1", V9000ToV9001)
	upgradepath.RegisterHandler("99.0.1", "99.0.0", V9991ToV9990)
}

func V9000ToV9001() error {
	ctx := handler.NewBackgroupContext()

	ctx.Logger.Infof("-------- start migrate workflow build job variable type --------")
	err := migrateWorkflowBuildJobVariableType(ctx)
	if err != nil {
		err = fmt.Errorf("failed to migrate workflow build job variable type, error: %w", err)
		ctx.Logger.Error(err)
		return err
	}

	return nil
}

func V9991ToV9990() error {
	return nil
}

func migrateWorkflowBuildJobVariableType(ctx *handler.Context) error {
	cursor, err := commonrepo.NewWorkflowV4Coll().ListByCursor(&commonrepo.ListWorkflowV4Option{})
	if err != nil {
		return fmt.Errorf("failed to list workflow by cursor, error: %v", err)
	}

	buildMap := make(map[string]*commonmodels.Build)
	buildTemplateMap := make(map[string]*commonmodels.BuildTemplate)

	getBuild := func(buildName string) (*commonmodels.Build, error) {
		if build, ok := buildMap[buildName]; ok {
			return build, nil
		}

		build, err := commonrepo.NewBuildColl().Find(&commonrepo.BuildFindOption{Name: buildName})
		if err != nil {
			return nil, fmt.Errorf("failed to find build %s, error: %v", buildName, err)
		}

		buildMap[buildName] = build
		return build, nil
	}

	updateBuild := func(build *commonmodels.Build) error {
		if err := commonrepo.NewBuildColl().Update(build); err != nil {
			return fmt.Errorf("failed to update build %s, error: %v", build.Name, err)
		}
		buildMap[build.Name] = build
		return nil
	}

	getBuildTemplate := func(buildTemplateID string) (*commonmodels.BuildTemplate, error) {
		if buildTemplate, ok := buildTemplateMap[buildTemplateID]; ok {
			return buildTemplate, nil
		}

		buildTemplate, err := commonrepo.NewBuildTemplateColl().Find(&commonrepo.BuildTemplateQueryOption{ID: buildTemplateID})
		if err != nil {
			return nil, fmt.Errorf("failed to find build template %s, error: %v", buildTemplateID, err)
		}

		buildTemplateMap[buildTemplateID] = buildTemplate
		return buildTemplate, nil
	}

	for cursor.Next(ctx) {
		var workflow commonmodels.WorkflowV4
		if err := cursor.Decode(&workflow); err != nil {
			return fmt.Errorf("failed to decode workflow, error: %v", err)
		}

		changed := false
		for _, stage := range workflow.Stages {
			for _, job := range stage.Jobs {
				if job.JobType == config.JobZadigBuild {
					buildSpec := &commonmodels.ZadigBuildJobSpec{}
					if err := commonmodels.IToi(job.Spec, buildSpec); err != nil {
						return fmt.Errorf("failed to convert job spec to ZadigBuildJobSpec, error: %v", err)
					}

					for _, svcBuild := range buildSpec.ServiceAndBuilds {
						for _, keyVal := range svcBuild.KeyVals {
							if keyVal.Type == "" {
								buildName := svcBuild.BuildName
								build, err := getBuild(buildName)
								if err != nil {
									return err
								}
								if build.TemplateID != "" {
									buildTemplate, err := getBuildTemplate(build.TemplateID)
									if err != nil {
										return err
									}
									if buildTemplate.PreBuild != nil {
										for _, env := range buildTemplate.PreBuild.Envs {
											if env.Key == keyVal.Key {
												keyVal.Type = env.Type
												changed = true
												break
											}
										}
									}
									for _, target := range build.Targets {
										if target.ServiceName == svcBuild.ServiceName && target.ServiceModule == svcBuild.ServiceModule {
											for _, env := range target.Envs {
												if env.Key == keyVal.Key {
													env.Type = keyVal.Type
													err = updateBuild(build)
													if err != nil {
														return err
													}
													break
												}
											}
										}
									}
								} else {
									build, err := getBuild(build.Name)
									if err != nil {
										return err
									}
									if build.PreBuild != nil {
										for _, env := range build.PreBuild.Envs {
											if env.Key == keyVal.Key {
												keyVal.Type = env.Type
												changed = true
												break
											}
										}
									}
								}
							}
						}
					}

					job.Spec = buildSpec

				}
			}
		}

		if changed {
			err = commonrepo.NewWorkflowV4Coll().Update(workflow.ID.Hex(), &workflow)
			if err != nil {
				return fmt.Errorf("failed to update workflow %s, error: %v", workflow.Name, err)
			}
		}
	}

	return nil
}
