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

package service

import (
	"fmt"
	"strconv"

	"github.com/koderover/zadig/v2/pkg/config"
	aslanconfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/larkplugin"
	"github.com/koderover/zadig/v2/pkg/tool/meego"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
	sdkcore "github.com/larksuite/project-oapi-sdk-golang/core"
	"github.com/larksuite/project-oapi-sdk-golang/v2/service/workitem"
)

func validateWorkflowForLarkPlugin(workflowName string) error {
	workflow, err := mongodb.NewWorkflowV4Coll().Find(workflowName)
	if err != nil {
		return fmt.Errorf("failed to find workflow %s: %w", workflowName, err)
	}

	var buildCount, deployCount, testCount, meegoCount int
	buildIdx, deployIdx := -1, -1
	jobIdx := 0

	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			switch job.JobType {
			case aslanconfig.JobZadigBuild:
				buildCount++
				buildIdx = jobIdx
			case aslanconfig.JobZadigDeploy:
				deployCount++
				deployIdx = jobIdx
			case aslanconfig.JobZadigTesting:
				testCount++
			case aslanconfig.JobMeegoTransition:
				meegoCount++
			default:
				return fmt.Errorf("workflow %s contains unsupported job type %s, only build, deploy, and test jobs are allowed", workflowName, job.JobType)
			}
			jobIdx++
		}
	}

	if buildCount != 1 {
		return fmt.Errorf("workflow %s must have exactly 1 build job, got %d", workflowName, buildCount)
	}
	if deployCount != 1 {
		return fmt.Errorf("workflow %s must have exactly 1 deploy job, got %d", workflowName, deployCount)
	}
	if testCount > 1 {
		return fmt.Errorf("workflow %s can have at most 1 test job, got %d", workflowName, testCount)
	}

	if deployIdx < buildIdx {
		return fmt.Errorf("workflow %s has invalid job order: deploy job must come after build job", workflowName)
	}

	return nil
}

// getWorkItemInfo gets the templateID/nodeID combination from the given work item
func getWorkItemInfo(ctx *internalhandler.Context, workspaceID, workItemType, workItemID string) (templateID int64, nodeID string, err error) {
	workItemIDInt, err := strconv.ParseInt(workItemID, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse workitem id: %w", err)
	}

	larkClient := larkplugin.NewClient(config.LarkPluginID(), config.LarkPluginSecret(), ctx.LarkPlugin.LarkType)
	larkResp, err := larkClient.ClientV2.WorkItem.GetWorkItemsByIds(ctx, workitem.NewGetWorkItemsByIdsReqBuilder().
		ProjectKey(workspaceID).
		WorkItemTypeKey(workItemType).
		WorkItemIDs([]int64{workItemIDInt}).
		Build(),
		sdkcore.WithAccessToken(ctx.LarkPlugin.PluginAccessToken),
		sdkcore.WithUserKey(ctx.LarkPlugin.UserKey),
	)
	if err != nil {
		return 0, "", fmt.Errorf("failed to get lark workitem: %w", err)
	}
	if larkResp.Code() != 0 {
		return 0, "", fmt.Errorf("failed to get lark workitem, code: %d, message: %s", larkResp.Code(), larkResp.ErrMsg)
	}
	if len(larkResp.Data) == 0 {
		return 0, "", fmt.Errorf("workitem could not be found")
	}

	workItem := larkResp.Data[0]

	var currentNodeIDs []string
	if *workItem.Pattern == string(meego.WorkItemPatternNode) {
		for _, currentNode := range workItem.CurrentNodes {
			currentNodeIDs = append(currentNodeIDs, *currentNode.ID)
		}
	} else if *workItem.Pattern == string(meego.WorkItemPatternState) {
		currentNodeIDs = append(currentNodeIDs, *workItem.WorkItemStatus.StateKey)
	} else {
		return 0, "", fmt.Errorf("unsupported pattern %s", *workItem.Pattern)
	}

	if len(currentNodeIDs) == 0 {
		return 0, "", fmt.Errorf("no current node found")
	}

	templateID = util.GetInt64FromPointer(workItem.TemplateID)
	nodeID = currentNodeIDs[0]

	return
}

// getWorkflowFromWorkItem gets the workflow from the given work item
func getWorkflowFromWorkItem(ctx *internalhandler.Context, workspaceID, workItemType, workItemID string) (*models.WorkflowV4, error) {
	templateID, nodeID, err := getWorkItemInfo(ctx, workspaceID, workItemType, workItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get work item info: %s", err)
	}

	config, err := mongodb.NewLarkPluginWorkflowConfigV2Coll().Find(workspaceID, workItemType, templateID, nodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	workflow, err := mongodb.NewWorkflowV4Coll().Find(config.WorkflowName)
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	return workflow, nil
}

// getFirstRepoFromWorkflow gets the first repo from the workflow's build job based on the service name and module
func getFirstRepoFromWorkflow(workflow *models.WorkflowV4, serviceName, serviceModule string) (*types.Repository, error) {
	var buildSpec *commonmodels.ZadigBuildJobSpec
	for _, stage := range workflow.Stages {
		for _, job := range stage.Jobs {
			if job.JobType == aslanconfig.JobZadigBuild {
				spec := &commonmodels.ZadigBuildJobSpec{}
				if err := commonmodels.IToiYaml(job.Spec, spec); err != nil {
					continue
				}
				buildSpec = spec
				break
			}
		}
		if buildSpec != nil {
			break
		}
	}
	if buildSpec == nil {
		return nil, fmt.Errorf("no build job found in workflow %s", workflow.Name)
	}

	var matchedBuild *commonmodels.ServiceAndBuild
	for _, opt := range buildSpec.ServiceAndBuildsOptions {
		if opt != nil && opt.ServiceName == serviceName && opt.ServiceModule == serviceModule {
			matchedBuild = opt
			break
		}
	}
	if matchedBuild == nil {
		return nil, fmt.Errorf("service %s/%s not found in workflow build options", serviceName, serviceModule)
	}

	buildSvc := commonservice.NewBuildService()
	buildInfo, err := buildSvc.GetBuild(matchedBuild.BuildName, serviceName, serviceModule)
	if err != nil {
		return nil, fmt.Errorf("failed to get build %s for %s/%s: %w", matchedBuild.BuildName, serviceName, serviceModule, err)
	}

	if len(buildInfo.Repos) == 0 {
		return nil, fmt.Errorf("no repos configured in build %s", matchedBuild.BuildName)
	}

	return buildInfo.Repos[0], nil
}
