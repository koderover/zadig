/*
Copyright 2022 The KodeRover Authors.

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

package webhook

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"go.uber.org/zap"

	microserviceConfig "github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/command"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	environmentservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/gitee"
	"github.com/koderover/zadig/v2/pkg/util"
)

func ProcessGiteeHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-Gitee-Token")
	secret := util.GetGitHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := gitee.HookEventType(req)
	event, err := gitee.ParseHook(eventType, payload)
	if err != nil {
		return err
	}

	baseURI, err := commonservice.GetSystemServerURL()
	if err != nil {
		return fmt.Errorf("failed to get system server URL: %w", err)
	}

	var errorList = &multierror.Error{}
	var wg sync.WaitGroup

	switch event := event.(type) {
	case *gitee.PushEvent:
		//add webhook user
		if len(event.Commits) > 0 {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  event.Commits[0].Author.Name,
				Email:     event.Commits[0].Author.Email,
				Source:    setting.SourceFromGitee,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}

		// FIXME: this func maybe panic if any errors occurred, just like gitee token expired
		if err := updateServiceTemplateByGiteeEvent(req.RequestURI, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
		// build webhook
		//wg.Add(1)
		//go func() {
		//	defer wg.Done()
		//	if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
		//		errorList = multierror.Append(errorList, err)
		//	}
		//}()

		//test webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerTestByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()

		//workflowv4 webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerWorkflowV4ByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
	case *gitee.PullRequestEvent:
		if event.Action != "open" && event.Action != "update" {
			return fmt.Errorf("action %s is skipped", event.Action)
		}

		if event.Action == "update" && event.ActionDesc == "target_branch_changed" {
			return fmt.Errorf("action %s is skipped", event.Action)
		}

		// build webhook
		//wg.Add(1)
		//go func() {
		//	defer wg.Done()
		//	if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
		//		errorList = multierror.Append(errorList, err)
		//	}
		//}()

		//test webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerTestByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
		//workflowv4 webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerWorkflowV4ByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
	case *gitee.TagPushEvent:
		// build webhook
		//wg.Add(1)
		//go func() {
		//	defer wg.Done()
		//	if err = TriggerWorkflowByGiteeEvent(event, baseURI, requestID, log); err != nil {
		//		errorList = multierror.Append(errorList, err)
		//	}
		//}()

		//test webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerTestByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
		//workflowv4 webhook
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err = TriggerWorkflowV4ByGiteeEvent(event, baseURI, requestID, log); err != nil {
				errorList = multierror.Append(errorList, err)
			}
		}()
	}
	wg.Wait()
	return errorList.ErrorOrNil()
}

func updateServiceTemplateByGiteeEvent(uri string, log *zap.SugaredLogger) error {
	svcTmplsMap := map[bool][]*commonmodels.Service{}
	serviceTmpls, err := GetGiteeTestingServiceTemplates()
	if err != nil {
		log.Errorf("failed to get gitee testing service templates, err: %s", err)
		return err
	}
	svcTmplsMap[false] = serviceTmpls
	productionServiceTmpls, err := GetGiteeProductionServiceTemplates()
	if err != nil {
		log.Errorf("failed to get gitee production service templates, err: %s", err)
		return err
	}
	svcTmplsMap[true] = productionServiceTmpls

	errs := &multierror.Error{}
	for production, serviceTmpls := range svcTmplsMap {
		for _, serviceTmpl := range serviceTmpls {
			serviceTmpl.Production = production

			service, err := commonservice.GetServiceTemplate(serviceTmpl.ServiceName, serviceTmpl.Type, serviceTmpl.ProductName, setting.ProductStatusDeleting, serviceTmpl.Revision, production, log)
			if err != nil {
				log.Errorf("failed to get gitee service templates, err:%s", err)
				errs = multierror.Append(errs, err)
			}
			detail, err := systemconfig.New().GetCodeHost(service.CodehostID)
			if err != nil {
				log.Errorf("failed to get codehost detail err:%s", err)
				errs = multierror.Append(errs, err)
			}
			newRepoName := fmt.Sprintf("%s-new", service.RepoName)
			if (time.Now().Unix() - detail.UpdatedAt) >= 86000 {
				token, err := gitee.RefreshAccessToken(detail.Address, detail.RefreshToken)
				if err == nil {
					detail.AccessToken = token.AccessToken
					detail.RefreshToken = token.RefreshToken
					detail.UpdatedAt = int64(token.CreatedAt)

					if err = systemconfig.New().UpdateCodeHost(detail.ID, detail); err != nil {
						log.Errorf("failed to updateCodeHost err:%s", err)
						errs = multierror.Append(errs, err)
						return errs.ErrorOrNil()
					}
				} else {
					log.Errorf("failed to refresh accessToken, err:%s", err)
					errs = multierror.Append(errs, err)
					return errs.ErrorOrNil()
				}
			}
			err = command.RunGitCmds(detail, service.RepoOwner, service.GetRepoNamespace(), newRepoName, service.BranchName, "origin")
			if err != nil {
				log.Errorf("failed to run git cmds err:%s", err)
				errs = multierror.Append(errs, err)
			}

			newBase, err := GetWorkspaceBasePath(newRepoName)
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			oldBase, err := GetWorkspaceBasePath(service.RepoName)
			if err != nil {
				errs = multierror.Append(errs, err)
				err = command.RunGitCmds(detail, service.RepoOwner, service.GetRepoNamespace(), service.RepoName, service.BranchName, "origin")
				if err != nil {
					errs = multierror.Append(errs, err)
				}
			}

			filePath, err := os.Stat(path.Join(newBase, service.LoadPath))
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			var newYamlContent string
			var oldYamlContent string
			if filePath.IsDir() {
				if newFileContents, err := readAllFileContentUnderDir(path.Join(newBase, service.LoadPath)); err == nil {
					newYamlContent = util.JoinYamls(newFileContents)
				} else {
					errs = multierror.Append(errs, err)
				}

				if oldFileContents, err := readAllFileContentUnderDir(path.Join(oldBase, service.LoadPath)); err == nil {
					oldYamlContent = util.JoinYamls(oldFileContents)
				} else {
					errs = multierror.Append(errs, err)
				}
			} else {
				if contentBytes, err := ioutil.ReadFile(path.Join(newBase, service.LoadPath)); err == nil {
					newYamlContent = string(contentBytes)
				} else {
					errs = multierror.Append(errs, err)
				}

				if contentBytes, err := ioutil.ReadFile(path.Join(oldBase, service.LoadPath)); err == nil {
					oldYamlContent = string(contentBytes)
				} else {
					errs = multierror.Append(errs, err)
				}
			}

			if strings.Compare(newYamlContent, oldYamlContent) != 0 {
				log.Infof("Started to sync service template %s from gitee %s", service.ServiceName, service.LoadPath)
				service.CreateBy = "system"
				service.Yaml = newYamlContent
				if err := SyncServiceTemplateFromGitee(service, log); err != nil {
					errs = multierror.Append(errs, err)
				}
			} else {
				log.Infof("Service template %s from gitee %s is not affected, no sync", service.ServiceName, service.LoadPath)
			}
		}
	}

	return errs.ErrorOrNil()
}

// GetGiteeTestingServiceTemplates Get all service templates maintained in gitee
func GetGiteeTestingServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGitee,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

// GetGiteeProductionServiceTemplates Get all service templates maintained in gitee
func GetGiteeProductionServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGitee,
	}
	return commonrepo.NewProductionServiceColl().ListMaxRevisions(opt)
}

func GetWorkspaceBasePath(repoName string) (string, error) {
	if strings.Contains(repoName, "/") {
		repoName = strings.Replace(repoName, "/", "-", -1)
	}
	base := path.Join(microserviceConfig.S3StoragePath(), repoName)

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, err
	}

	return base, nil
}

func SyncServiceTemplateFromGitee(service *commonmodels.Service, log *zap.SugaredLogger) error {
	if service.Source != setting.SourceFromGitee {
		return fmt.Errorf("SyncServiceTemplateFromGitee service template is not from gitee,source:%s", service.Source)
	}

	if _, err := syncGiteeLatestCommit(service); err != nil {
		log.Errorf("SyncServiceTemplateFromGitee sync change log from gitee failed service %s, error: %s", service.ServiceName, err)
		return err
	}

	if service.Type == setting.K8SDeployType {
		if err := ensureServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
			log.Errorf("SyncServiceTemplateFromGitee ensureServiceTmpl error: %s", err)
			return e.ErrValidateTemplate.AddDesc(err.Error())
		}

		if err := repository.Create(service, service.Production); err != nil {
			log.Errorf("SyncServiceTemplateFromGitee Failed to sync service %s from gitee path %s error: %s", service.ServiceName, service.SrcPath, err)
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

		return environmentservice.AutoDeployYamlServiceToEnvs(service.CreateBy, "", service, service.Production, log)
	}
	// remove old repo dir
	oldRepoDir := filepath.Join(microserviceConfig.S3StoragePath(), service.RepoName)
	os.RemoveAll(oldRepoDir)
	// copy new repo data to old dir data
	if err := os.MkdirAll(oldRepoDir, 0775); err != nil {
		return err
	}

	newRepoDir := filepath.Join(microserviceConfig.S3StoragePath(), service.RepoName+"-new")
	if copyErr := copy.Copy(newRepoDir, oldRepoDir); copyErr != nil {
		return copyErr
	}

	if err := reloadServiceTmplFromGitee(service, log); err != nil {
		return err
	}
	log.Infof("End of sync service template %s from gitee path %s", service.ServiceName, service.SrcPath)
	return nil
}

func syncGiteeLatestCommit(service *commonmodels.Service) (*systemconfig.CodeHost, error) {
	if service.CodehostID == 0 {
		return nil, fmt.Errorf("codehostId cannot be empty")
	}
	if service.RepoName == "" {
		return nil, fmt.Errorf("repoName cannot be empty")
	}
	if service.BranchName == "" {
		return nil, fmt.Errorf("branchName cannot be empty")
	}
	detail, err := systemconfig.New().GetCodeHost(service.CodehostID)
	if err != nil {
		return nil, err
	}

	giteeCli := gitee.NewClient(detail.ID, detail.Address, detail.AccessToken, microserviceConfig.ProxyHTTPSAddr(), detail.EnableProxy)
	commit, err := giteeCli.GetSingleBranch(detail.Address, detail.AccessToken, service.RepoOwner, service.RepoName, service.BranchName)
	if err != nil {
		return detail, err
	}

	service.Commit = &commonmodels.Commit{
		SHA:     commit.Commit.Sha,
		Message: commit.Commit.Commit.Message,
	}
	return detail, nil
}

func reloadServiceTmplFromGitee(svc *commonmodels.Service, log *zap.SugaredLogger) error {
	_, err := service.CreateOrUpdateHelmServiceFromRepo(svc.ProductName, &service.HelmServiceCreationArgs{
		HelmLoadSource: service.HelmLoadSource{
			Source: setting.SourceFromGitee,
		},
		CreatedBy: svc.CreateBy,
		CreateFrom: &service.CreateFromRepo{
			CodehostID: svc.CodehostID,
			Owner:      svc.RepoOwner,
			Repo:       svc.RepoName,
			Branch:     svc.BranchName,
			Paths:      []string{svc.LoadPath},
		},
		Production: svc.Production,
	}, true, log)
	return err
}
