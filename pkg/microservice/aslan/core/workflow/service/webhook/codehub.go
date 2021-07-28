package webhook

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	gitservice "github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/git"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/codehub"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ProcessCodehubHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-Codehub-Token")
	secret := gitservice.GetHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := codehub.HookEventType(req)
	event, err := codehub.ParseHook(eventType, payload)
	if err != nil {
		log.Warnf("unexpected event type: %s", eventType)
		return nil
	}

	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseURI := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	var pushEvent *codehub.PushEvent
	var errorList = &multierror.Error{}
	switch event := event.(type) {
	case *codehub.PushEvent:
		pushEvent = event
		if len(pushEvent.Commits) > 0 {
			webhookUser := &commonmodels.WebHookUser{
				Domain:    req.Header.Get("X-Forwarded-Host"),
				UserName:  pushEvent.Commits[0].Author.Name,
				Email:     pushEvent.Commits[0].Author.Email,
				Source:    setting.SourceFromCodeHub,
				CreatedAt: time.Now().Unix(),
			}
			commonrepo.NewWebHookUserColl().Upsert(webhookUser)
		}

		if err = updateServiceTemplateByCodehubPushEvent(pushEvent, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	//产品工作流webhook
	if err = TriggerWorkflowByCodehubEvent(event, baseURI, requestID, log); err != nil {
		errorList = multierror.Append(errorList, err)
	}

	return errorList.ErrorOrNil()
}

func updateServiceTemplateByCodehubPushEvent(event *codehub.PushEvent, log *zap.SugaredLogger) error {
	log.Infof("EVENT: CODEHUB WEBHOOK UPDATING SERVICE TEMPLATE")
	serviceTmpls, err := GetCodehubServiceTemplates()
	if err != nil {
		log.Errorf("Failed to get codehub service templates, error: %v", err)
		return err
	}

	errs := &multierror.Error{}
	for _, service := range serviceTmpls {
		srcPath := service.SrcPath
		_, _, _, _, path, _, err := GetOwnerRepoBranchPath(srcPath)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		// 判断PushEvent的Diffs中是否包含该服务模板的src_path
		affected := false
		fileNames := []string{}
		for _, commit := range event.Commits {
			fileNames = append(fileNames, commit.Added...)
			fileNames = append(fileNames, commit.Modified...)
			fileNames = append(fileNames, commit.Removed...)
		}

		for _, fileName := range fileNames {
			if strings.Contains(fileName, path) {
				affected = true
				break
			}
		}

		if affected {
			log.Infof("Started to sync service template %s from codehub %s", service.ServiceName, service.SrcPath)
			//TODO: 异步处理
			service.CreateBy = "system"
			err := SyncServiceTemplateFromCodehub(service, log)
			if err != nil {
				log.Errorf("SyncServiceTemplateFromCodehub failed, error: %v", err)
				errs = multierror.Append(errs, err)
			}
		} else {
			log.Infof("Service template %s from codehub %s is not affected, no sync", service.ServiceName, service.SrcPath)
		}

	}
	return errs.ErrorOrNil()
}

func GetCodehubServiceTemplates() ([]*commonmodels.Service, error) {
	opt := &commonrepo.ServiceFindOption{
		Type:          setting.K8SDeployType,
		Source:        setting.SourceFromCodeHub,
		ExcludeStatus: setting.ProductStatusDeleting,
	}
	return commonrepo.NewServiceColl().List(opt)
}

// SyncServiceTemplateFromCodehub Force to sync Service Template to latest commit and content,
// Notes: if remains the same, quit sync; if updates, revision +1
func SyncServiceTemplateFromCodehub(service *commonmodels.Service, log *zap.SugaredLogger) error {
	// 判断一下Source字段，如果Source字段不是gitlab，直接返回
	if service.Source != setting.SourceFromCodeHub {
		return fmt.Errorf("service template is not from codehub")
	}
	// 获取当前Commit的SHA
	var before string
	if service.Commit != nil {
		before = service.Commit.SHA
	}
	// Sync最新的Commit的SHA
	var after string
	err := syncCodehubLatestCommit(service)
	if err != nil {
		return err
	}
	after = service.Commit.SHA
	// 判断一下是否需要Sync内容
	if before == after {
		log.Infof("Before and after SHA: %s remains the same, no need to sync", before)
		// 无需更新
		return nil
	}
	// 在Ensure过程中会检查source，如果source为 codehub，则同步 codehub 内容到service中
	if err := fillServiceTmpl(setting.WebhookTaskCreator, service, log); err != nil {
		log.Errorf("ensureServiceTmpl error: %+v", err)
		return e.ErrValidateTemplate.AddDesc(err.Error())
	}
	// 更新到数据库，revision+1
	if err := commonrepo.NewServiceColl().Create(service); err != nil {
		log.Errorf("Failed to sync service %s from codehub path %s error: %v", service.ServiceName, service.SrcPath, err)
		return e.ErrCreateTemplate.AddDesc(err.Error())
	}
	log.Infof("End of sync service template %s from codehub path %s", service.ServiceName, service.SrcPath)
	return nil
}
