package webhook

import (
	"encoding/json"
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
	e "github.com/koderover/zadig/pkg/tool/errors"
)

// EventType represents a Codehub event type.
type EventType string

// List of available event types.
const (
	EventTypeMergeRequest EventType = "Merge Request Hook"
	EventTypePush         EventType = "Push Hook"
	EventTypeTagPush      EventType = "Tag Push Hook"
)

const eventTypeHeader = "X-Codehub-Event"

// HookEventType returns the event type for the given request.
func HookEventType(r *http.Request) EventType {
	return EventType(r.Header.Get(eventTypeHeader))
}

func parseHook(eventType EventType, payload []byte) (event interface{}, err error) {
	return parseWebhook(eventType, payload)
}

type User struct {
	Name      string `json:"name"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

type Project struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type MergeParams struct {
	ForceRemoveSourceBranch bool `json:"force_remove_source_branch"`
}

type Source struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type Target struct {
	ID                int         `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	WebURL            string      `json:"web_url"`
	AvatarURL         interface{} `json:"avatar_url"`
	GitSSHURL         string      `json:"git_ssh_url"`
	GitHTTPURL        string      `json:"git_http_url"`
	Namespace         string      `json:"namespace"`
	VisibilityLevel   int         `json:"visibility_level"`
	PathWithNamespace string      `json:"path_with_namespace"`
	DefaultBranch     string      `json:"default_branch"`
	CiConfigPath      interface{} `json:"ci_config_path"`
	Homepage          string      `json:"homepage"`
	URL               string      `json:"url"`
	SSHURL            string      `json:"ssh_url"`
	HTTPURL           string      `json:"http_url"`
}

type LastCommit struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	URL       string    `json:"url"`
	Author    Author    `json:"author"`
}

type ObjectAttributes struct {
	AssigneeID                interface{} `json:"assignee_id"`
	AuthorID                  int         `json:"author_id"`
	CreatedAt                 string      `json:"created_at"`
	Description               string      `json:"description"`
	HeadPipelineID            interface{} `json:"head_pipeline_id"`
	ID                        int         `json:"id"`
	IID                       int         `json:"iid"`
	LastEditedAt              interface{} `json:"last_edited_at"`
	LastEditedByID            interface{} `json:"last_edited_by_id"`
	MergeCommitSha            interface{} `json:"merge_commit_sha"`
	MergeError                interface{} `json:"merge_error"`
	MergeParams               MergeParams `json:"merge_params"`
	MergeStatus               string      `json:"merge_status"`
	MergeUserID               interface{} `json:"merge_user_id"`
	MergeWhenPipelineSucceeds bool        `json:"merge_when_pipeline_succeeds"`
	MilestoneID               interface{} `json:"milestone_id"`
	SourceBranch              string      `json:"source_branch"`
	SourceProjectID           int         `json:"source_project_id"`
	State                     string      `json:"state"`
	TargetBranch              string      `json:"target_branch"`
	TargetProjectID           int         `json:"target_project_id"`
	TimeEstimate              int         `json:"time_estimate"`
	Title                     string      `json:"title"`
	UpdatedAt                 string      `json:"updated_at"`
	UpdatedByID               interface{} `json:"updated_by_id"`
	URL                       string      `json:"url"`
	Source                    Source      `json:"source"`
	Target                    Target      `json:"target"`
	LastCommit                LastCommit  `json:"last_commit"`
	WorkInProgress            bool        `json:"work_in_progress"`
	TotalTimeSpent            int         `json:"total_time_spent"`
	HumanTotalTimeSpent       interface{} `json:"human_total_time_spent"`
	HumanTimeEstimate         interface{} `json:"human_time_estimate"`
	Action                    string      `json:"action"`
}

type MergeEvent struct {
	ObjectKind       string           `json:"object_kind"`
	EventType        string           `json:"event_type"`
	User             User             `json:"user"`
	Project          Project          `json:"project"`
	ObjectAttributes ObjectAttributes `json:"object_attributes"`
	Labels           []interface{}    `json:"labels"`
}

type PushEvent struct {
	ObjectKind        string      `json:"object_kind"`
	EventName         string      `json:"event_name"`
	Before            string      `json:"before"`
	After             string      `json:"after"`
	Ref               string      `json:"ref"`
	CheckoutSha       string      `json:"checkout_sha"`
	Message           interface{} `json:"message"`
	UserID            int         `json:"user_id"`
	UserName          string      `json:"user_name"`
	UserUsername      string      `json:"user_username"`
	UserEmail         string      `json:"user_email"`
	UserAvatar        string      `json:"user_avatar"`
	ProjectID         int         `json:"project_id"`
	Project           Project     `json:"project"`
	Commits           []Commit    `json:"commits"`
	TotalCommitsCount int         `json:"total_commits_count"`
}

type Commit struct {
	ID        string        `json:"id"`
	Message   string        `json:"message"`
	Timestamp time.Time     `json:"timestamp"`
	URL       string        `json:"url"`
	Author    Author        `json:"author"`
	Added     []interface{} `json:"added"`
	Modified  []string      `json:"modified"`
	Removed   []interface{} `json:"removed"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func parseWebhook(eventType EventType, payload []byte) (event interface{}, err error) {
	switch eventType {
	case EventTypeMergeRequest:
		event = &MergeEvent{}
	case EventTypePush:
		event = &PushEvent{}
	default:
		return nil, fmt.Errorf("unexpected event type: %s", eventType)
	}

	if err := json.Unmarshal(payload, event); err != nil {
		return nil, err
	}

	return event, nil
}

func ProcessCodehubHook(payload []byte, req *http.Request, requestID string, log *zap.SugaredLogger) error {
	token := req.Header.Get("X-Codehub-Token")
	secret := gitservice.GetHookSecret()

	if secret != "" && token != secret {
		return errors.New("token is illegal")
	}

	eventType := HookEventType(req)
	event, err := parseHook(eventType, payload)
	if err != nil {
		return err
	}

	forwardedProto := req.Header.Get("X-Forwarded-Proto")
	forwardedHost := req.Header.Get("X-Forwarded-Host")
	baseURI := fmt.Sprintf("%s://%s", forwardedProto, forwardedHost)

	var pushEvent *PushEvent
	var mergeEvent *MergeEvent
	var errorList = &multierror.Error{}

	switch event := event.(type) {
	case *PushEvent:
		pushEvent = event
	case *MergeEvent:
		mergeEvent = event
	}

	//触发工作流webhook和测试管理webhook
	if pushEvent != nil {
		//add webhook user
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

		//产品工作流webhook
		if err = TriggerWorkflowByCodehubEvent(pushEvent, baseURI, requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}

		if err = updateServiceTemplateByCodehubPushEvent(pushEvent, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	if mergeEvent != nil {
		//产品工作流webhook
		if err = TriggerWorkflowByCodehubEvent(mergeEvent, baseURI, requestID, log); err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	return errorList.ErrorOrNil()
}

func updateServiceTemplateByCodehubPushEvent(event *PushEvent, log *zap.SugaredLogger) error {
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
		for _, commit := range event.Commits {
			fileNames := commit.Modified
			for _, fileName := range fileNames {
				if strings.Contains(fileName, path) || strings.Contains(fileName, path) {
					affected = true
					break
				}
			}
			if affected {
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
