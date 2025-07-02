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

package webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/otiai10/copy"
	"go.uber.org/zap"

	systemConfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/command"
	gerritservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/gerrit"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	environmentservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/environment/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/service/service"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/systemconfig"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/gerrit"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	changeMergedEventType    = "change-merged"
	patchsetCreatedEventType = "patchset-created"
)

type gerritTypeEvent struct {
	Type           string `json:"type"`
	EventCreatedOn int    `json:"eventCreatedOn"`
}

func ProcessGerritHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	baseURI := systemConfig.SystemAddress()
	gerritTypeEventObj := new(gerritTypeEvent)
	if err := json.Unmarshal(payload, gerritTypeEventObj); err != nil {
		log.Errorf("processGerritHook json.Unmarshal err : %v", err)
		return fmt.Errorf("this event is not supported")
	}
	if gerritTypeEventObj.Type == changeMergedEventType {
		err := updateServiceTemplateByGerritEvent(req.RequestURI, log)
		if err != nil {
			log.Errorf("updateServiceTemplateByGerritEvent err : %v", err)
		}
	}
	var wg sync.WaitGroup
	var errorList = &multierror.Error{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := TriggerWorkflowV4ByGerritEvent(gerritTypeEventObj, payload, req.RequestURI, baseURI, req.Header.Get("X-Forwarded-Host"), requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}()
	wg.Wait()
	return errorList.ErrorOrNil()
}

func updateServiceTemplateByGerritEvent(uri string, log *zap.SugaredLogger) error {
	svcTmplsMap := map[bool][]*commonmodels.Service{}
	serviceTmpls, err := GetGerritTestingServiceTemplates()
	log.Infof("updateServiceTemplateByGerritEvent service templates: %v", serviceTmpls)
	if err != nil {
		log.Errorf("updateServiceTemplateByGerritEvent Failed to get gerrit testing service templates, error: %v", err)
		return err
	}
	svcTmplsMap[false] = serviceTmpls
	productionServiceTmpls, err := GetGerritProductionServiceTemplates()
	log.Infof("updateServiceTemplateByGerritEvent service templates: %v", serviceTmpls)
	if err != nil {
		log.Errorf("updateServiceTemplateByGerritEvent Failed to get gerrit production service templates, error: %v", err)
		return err
	}
	svcTmplsMap[true] = productionServiceTmpls

	errs := &multierror.Error{}
	for production, serviceTmpls := range svcTmplsMap {
		for _, serviceTmpl := range serviceTmpls {
			serviceTmpl.Production = production

			if strings.Contains(uri, "?") {
				if !strings.Contains(uri, serviceTmpl.ServiceName) {
					continue
				}
			}

			service, err := commonservice.GetServiceTemplate(serviceTmpl.ServiceName, serviceTmpl.Type, serviceTmpl.ProductName, setting.ProductStatusDeleting, serviceTmpl.Revision, production, log)
			if err != nil {
				log.Errorf("updateServiceTemplateByGerritEvent GetServiceTemplate err:%v", err)
				errs = multierror.Append(errs, err)
			}
			detail, err := systemconfig.New().GetCodeHost(service.GerritCodeHostID)
			if err != nil {
				log.Errorf("updateServiceTemplateByGerritEvent GetCodehostDetail err:%v", err)
				errs = multierror.Append(errs, err)
			}
			newRepoName := fmt.Sprintf("%s-new", service.GerritRepoName)
			err = command.RunGitCmds(detail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, newRepoName, service.GerritBranchName, service.GerritRemoteName)
			if err != nil {
				log.Errorf("updateServiceTemplateByGerritEvent runGitCmds err:%v", err)
				errs = multierror.Append(errs, err)
			}

			newBase, err := GetGerritWorkspaceBasePath(newRepoName)
			if err != nil {
				errs = multierror.Append(errs, err)
			}

			oldBase, err := GetGerritWorkspaceBasePath(service.GerritRepoName)
			if err != nil {
				errs = multierror.Append(errs, err)
				err = command.RunGitCmds(detail, setting.GerritDefaultOwner, setting.GerritDefaultOwner, service.GerritRepoName, service.GerritBranchName, service.GerritRemoteName)
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

			if strings.Compare(newYamlContent, oldYamlContent) != 0 || service.Type == setting.HelmDeployType {
				log.Infof("Started to sync service template %s from gerrit %s", service.ServiceName, service.LoadPath)
				service.CreateBy = "system"
				service.Yaml = newYamlContent
				if err := SyncServiceTemplateFromGerrit(service, log); err != nil {
					errs = multierror.Append(errs, err)
				}
			} else {
				log.Infof("Service template %s from gerrit %s is not affected, no sync", service.ServiceName, service.LoadPath)
			}
		}
	}

	return errs.ErrorOrNil()
}

func GetGerritWorkspaceBasePath(repoName string) (string, error) {
	return gerritservice.GetGerritWorkspaceBasePath(repoName)
}

// GetGerritTestingServiceTemplates Get all service templates maintained in gerrit
func GetGerritTestingServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGerrit,
	}
	return commonrepo.NewServiceColl().ListMaxRevisions(opt)
}

// GetGerritProductionServiceTemplates Get all service templates maintained in gerrit
func GetGerritProductionServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceListOption{
		Source: setting.SourceFromGerrit,
	}
	return commonrepo.NewProductionServiceColl().ListMaxRevisions(opt)
}

// SyncServiceTemplateFromGerrit Force to sync Service Template to the latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromGerrit(service *commonmodels.Service, log *zap.SugaredLogger) error {
	if service.Source != setting.SourceFromGerrit {
		return fmt.Errorf("SyncServiceTemplateFromGerrit Service template is not from gerrit")
	}

	// Sync commit information
	if _, err := syncGerritLatestCommit(service); err != nil {
		log.Errorf("SyncServiceTemplateFromGerrit Sync change log from gerrit failed service %s, error: %v", service.ServiceName, err)
		return err
	}

	if service.Type == setting.K8SDeployType {
		// During the Ensure process, the source will be checked. If the source is gerrit, the gerrit content will be synchronized to the service.
		if err := ensureServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
			log.Errorf("SyncServiceTemplateFromGerrit ensureServiceTmpl error: %+v", err)
			return e.ErrValidateTemplate.AddDesc(err.Error())
		}
		// Update to database, revision+1
		if err := repository.Create(service, service.Production); err != nil {
			log.Errorf("SyncServiceTemplateFromGerrit Failed to sync service %s from gerrit path %s error: %v", service.ServiceName, service.SrcPath, err)
			return e.ErrCreateTemplate.AddDesc(err.Error())
		}

		return environmentservice.AutoDeployYamlServiceToEnvs(service.CreateBy, "", service, service.Production, log)
	}
	// remove old repo dir
	oldRepoDir := filepath.Join(config.S3StoragePath(), service.GerritRepoName)
	os.RemoveAll(oldRepoDir)
	// copy new repo data to old dir data
	if err := os.MkdirAll(oldRepoDir, 0775); err != nil {
		return err
	}

	newRepoDir := filepath.Join(config.S3StoragePath(), service.GerritRepoName+"-new")
	if copyErr := copy.Copy(newRepoDir, oldRepoDir); copyErr != nil {
		return copyErr
	}

	if err := reloadServiceTmplFromGerrit(service, log); err != nil {
		return err
	}
	log.Infof("End of sync service template %s from gerrit path %s", service.ServiceName, service.SrcPath)
	return nil
}

func syncGerritLatestCommit(service *commonmodels.Service) (*systemconfig.CodeHost, error) {
	if service.GerritCodeHostID == 0 {
		return nil, fmt.Errorf("codehostId不能是空的")
	}
	if service.GerritRepoName == "" {
		return nil, fmt.Errorf("repoName不能是空的")
	}
	if service.GerritBranchName == "" {
		return nil, fmt.Errorf("branchName不能是空的")
	}
	detail, err := systemconfig.New().GetCodeHost(service.GerritCodeHostID)
	if err != nil {
		return nil, err
	}

	gerritCli := gerrit.NewClient(detail.Address, detail.AccessToken, config.ProxyHTTPSAddr(), detail.EnableProxy)
	commit, err := gerritCli.GetCommitByBranch(service.GerritRepoName, service.GerritBranchName)
	if err != nil {
		return detail, err
	}

	service.Commit = &commonmodels.Commit{
		SHA:     commit.Commit,
		Message: commit.Message,
	}
	return detail, nil
}

// ensureServiceTmpl 检查服务模板参数
func ensureServiceTmpl(userName string, args *commonmodels.Service, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("service template arg is null")
	}
	if len(args.ServiceName) == 0 {
		return errors.New("service name is empty")
	}
	if !config.ServiceNameRegex.MatchString(args.ServiceName) {
		return fmt.Errorf("导入的文件目录和文件名称仅支持字母，数字， 中划线和 下划线")
	}
	if args.Type == setting.K8SDeployType {
		if args.Containers == nil {
			args.Containers = make([]*commonmodels.Container, 0)
		}
		// 配置来源为Gitlab，需要从Gitlab同步配置，并设置KubeYamls.
		if args.Source != setting.SourceFromGithub && args.Source != setting.SourceFromGitlab {
			// 拆分 all-in-one yaml文件
			// 替换分隔符
			args.Yaml = util.ReplaceWrapLine(args.Yaml)
			// 分隔符为\n---\n
			args.KubeYamls = util.SplitYaml(args.Yaml)
		}

		// 遍历args.KubeYamls，获取 Deployment 或者 StatefulSet 里面所有containers 镜像和名称
		if err := commonutil.SetCurrentContainerImages(args); err != nil {
			return err
		}

		log.Infof("find %d containers in service %s", len(args.Containers), args.ServiceName)
	}

	// 设置新的版本号
	rev, err := commonutil.GenerateServiceNextRevision(args.Production, args.ServiceName, args.ProductName)
	if err != nil {
		return fmt.Errorf("get next service template revision error: %v", err)
	}

	args.Revision = rev

	return nil
}

func reloadServiceTmplFromGerrit(svc *commonmodels.Service, log *zap.SugaredLogger) error {
	_, err := service.CreateOrUpdateHelmServiceFromRepo(svc.ProductName, &service.HelmServiceCreationArgs{
		HelmLoadSource: service.HelmLoadSource{
			Source: service.LoadFromGerrit,
		},
		CreatedBy: svc.CreateBy,
		CreateFrom: &service.CreateFromRepo{
			CodehostID: svc.CodehostID,
			Owner:      svc.RepoOwner,
			Namespace:  svc.GetRepoNamespace(),
			Repo:       svc.RepoName,
			Branch:     svc.BranchName,
			Paths:      []string{svc.LoadPath},
		},
		Production: svc.Production,
	}, true, log)
	return err
}

// Get the contents of all files in a directory
func readAllFileContentUnderDir(localBase string) ([]string, error) {
	fileTree := os.DirFS(localBase)
	allFileContents := []string{}
	err := fs.WalkDir(fileTree, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		mode := entry.Type()
		switch {
		case mode.IsRegular():
			fileContents, err := fs.ReadFile(fileTree, path)
			if err != nil {
				return err
			}

			allFileContents = append(allFileContents, string(fileContents))
			return nil
		default:
			return nil
		}
	})
	return allFileContents, err
}
