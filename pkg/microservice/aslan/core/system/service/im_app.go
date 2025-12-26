/*
 * Copyright 2022 The KodeRover Authors.
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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	larkservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/lark"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
	"github.com/koderover/zadig/v2/pkg/util"
)

// TODO: update this logic to support dynamic connection handling for multiple instances
func InitSSEConnections() error {
	resp, err := mongodb.NewIMAppColl().List(context.Background(), "")
	if err != nil {
		log.Errorf("list external approval error: %v", err)
		return e.ErrListIMApp.AddErr(err)
	}

	for _, imApp := range resp {
		if imApp.Type == setting.IMLark || imApp.Type == setting.IMLarkIntl {
			err := CreateLarkSSEConnection(imApp)
			if err != nil {
				log.Errorf("failed to creates sse connection for lark, error: %s")
				return err
			}
		}
	}

	return nil
}

func ListIMApp(_type string, log *zap.SugaredLogger) ([]*commonmodels.IMApp, error) {
	resp, err := mongodb.NewIMAppColl().List(context.Background(), _type)
	if err != nil {
		log.Errorf("list external approval error: %v", err)
		return nil, e.ErrListIMApp.AddErr(err)
	}

	return resp, nil
}

func CreateIMApp(args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	switch args.Type {
	case setting.IMDingTalk:
		return createDingTalkIMApp(args, log)
	case setting.IMLark, setting.IMLarkIntl:
		return createLarkIMApp(args, log)
	case setting.IMWorkWx:
		return createWorkWxIMApp(args, log)
	default:
		return errors.Errorf("unknown im type %s", args.Type)
	}
}

func createDingTalkIMApp(args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	if err := dingtalk.Validate(args.DingTalkAppKey, args.DingTalkAppSecret); err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	client := dingtalk.NewClient(args.DingTalkAppKey, args.DingTalkAppSecret)
	list, err := client.GetAllApprovalFormDefinitionList()
	if err != nil {
		return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "get all approval form definition list"))
	}
	// check dingtalk app has default approval form
	for _, form := range list {
		if form.Name == dingtalk.DefaultApprovalFormName {
			args.DingTalkDefaultApprovalFormCode = form.ProcessCode
			break
		}
	}

	if args.DingTalkDefaultApprovalFormCode == "" {
		resp, err := client.CreateApproval()
		if err != nil {
			return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "create approval form"))
		}
		args.DingTalkDefaultApprovalFormCode = resp.ProcessCode
	}

	_, err = mongodb.NewIMAppColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create dingtalk IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}
	return nil
}

func createLarkIMApp(args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	err := lark.Validate(args.AppID, args.AppSecret, args.Type)
	if err != nil {
		return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	_, err = mongodb.NewIMAppColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create lark IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}

	err = CreateLarkSSEConnection(args)
	if err != nil {
		log.Errorf("create lark IM SSEConnection error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}
	return nil
}

func createWorkWxIMApp(args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	client := workwx.NewClient(args.Host, args.CorpID, args.AgentID, args.AgentSecret)

	templateName, controls := generateWorkWXDefaultApprovalTemplate(args.Name)
	templateID, err := client.CreateApprovalTemplate(templateName, controls)
	if err != nil {
		log.Errorf("failed to create approval template for workwx, error: %s", err)
		return fmt.Errorf("failed to create approval template for workwx, error: %s", err)
	}

	args.WorkWXApprovalTemplateID = templateID

	_, err = mongodb.NewIMAppColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create workwx IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}
	return nil
}

func UpdateIMApp(id string, args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	switch args.Type {
	case setting.IMDingTalk:
		return updateDingTalkIMApp(id, args, log)
	case setting.IMLark, setting.IMLarkIntl:
		return updateLarkIMApp(id, args, log)
	case setting.IMWorkWx:
		return updateWorkWxIMApp(id, args, log)
	default:
		return errors.Errorf("unknown im type %s", args.Type)
	}
}

func updateDingTalkIMApp(id string, args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	if err := dingtalk.Validate(args.DingTalkAppKey, args.DingTalkAppSecret); err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	client := dingtalk.NewClient(args.DingTalkAppKey, args.DingTalkAppSecret)

	list, err := client.GetAllApprovalFormDefinitionList()
	if err != nil {
		return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "get all approval form definition list"))
	}
	// check dingtalk app has default approval form
	for _, form := range list {
		if form.Name == dingtalk.DefaultApprovalFormName {
			args.DingTalkDefaultApprovalFormCode = form.ProcessCode
			break
		}
	}
	if args.DingTalkDefaultApprovalFormCode == "" {
		resp, err := client.CreateApproval()
		if err != nil {
			return e.ErrUpdateIMApp.AddErr(errors.Wrap(err, "create approval form"))
		}
		args.DingTalkDefaultApprovalFormCode = resp.ProcessCode
	}

	err = mongodb.NewIMAppColl().Update(context.Background(), id, args)
	if err != nil {
		return errors.Wrap(err, "update dingtalk info with process code")
	}
	return nil
}

func updateLarkIMApp(id string, args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	err := lark.Validate(args.AppID, args.AppSecret, args.Type)
	if err != nil {
		return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	err = mongodb.NewIMAppColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update lark IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}

	err = CreateLarkSSEConnection(args)
	if err != nil {
		log.Errorf("create lark IM SSEConnection error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}
	return nil
}

func updateWorkWxIMApp(id string, args *commonmodels.IMApp, log *zap.SugaredLogger) error {
	err := workwx.Validate(args.Host, args.CorpID, args.AgentID, args.AgentSecret)
	if err != nil {
		return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	err = mongodb.NewIMAppColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update lark IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}
	return nil
}

func DeleteIMApp(id string, log *zap.SugaredLogger) error {
	err := mongodb.NewIMAppColl().DeleteByID(context.Background(), id)
	if err != nil {
		log.Errorf("delete external approval error: %v", err)
		return e.ErrDeleteIMApp.AddErr(err)
	}
	return nil
}

func ValidateIMApp(im *commonmodels.IMApp, log *zap.SugaredLogger) error {
	switch im.Type {
	case setting.IMLark, setting.IMLarkIntl:
		return lark.Validate(im.AppID, im.AppSecret, im.Type)
	case setting.IMDingTalk:
		return dingtalk.Validate(im.DingTalkAppKey, im.DingTalkAppSecret)
	case setting.IMWorkWx:
		return workwx.Validate(im.Host, im.CorpID, im.AgentID, im.AgentSecret)
	default:
		return e.ErrValidateIMApp.AddDesc("invalid type")
	}
}

func generateWorkWXDefaultApprovalTemplate(name string) ([]*workwx.GeneralText, []*workwx.ApprovalControl) {
	templateName := make([]*workwx.GeneralText, 0)
	templateName = append(templateName, &workwx.GeneralText{
		Text: fmt.Sprintf("%s - %s", "Zadig 审批", name),
		Lang: "zh_CN",
	})

	controls := make([]*workwx.ApprovalControl, 0)

	controls = append(controls, &workwx.ApprovalControl{Property: &workwx.ApprovalControlProperty{
		Type: config.DefaultWorkWXApprovalControlType,
		ID:   config.DefaultWorkWXApprovalControlID,
		Title: []*workwx.GeneralText{
			{
				Text: "审批内容",
				Lang: "zh_CN",
			},
		},
		Require: 1,
		UnPrint: 0,
	}})

	return templateName, controls
}

const (
	larkSSELockFormat = "LARK_SSE_LOCK_%s"
)

func CreateLarkSSEConnection(arg *commonmodels.IMApp) error {
	if arg.Type != setting.IMLark && arg.Type != setting.IMLarkIntl {
		return fmt.Errorf("invalid type: %s to create lark sse connection", arg.Type)
	}

	// nothing to create
	if arg.LarkEventType != setting.LarkEventTypeSSE {
		return nil
	}

	lock := cache.NewRedisLock(fmt.Sprintf(larkSSELockFormat, arg.ID.Hex()))
	err := lock.TryLock()
	if err != nil {
		return fmt.Errorf("Lark sse lock for %s is occupied by another instance: err: %s", arg.Name, err)
	}

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnCustomizedEvent("approval_task", larkSSEHandler).     // 审批任务状态变更
		OnCustomizedEvent("approval_instance", larkSSEHandler). // 审批实例事件
		OnCustomizedEvent("approval", larkSSEHandler)

	eventHandler.InitConfig()

	cli := larkws.NewClient(arg.AppID, arg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithDomain(lark.GetLarkBaseUrl(arg.Type)),
	)

	util.Go(
		func() {
			cli.Start(context.Background())
		},
	)

	return nil
}

func larkSSEHandler(ctx context.Context, event *larkevent.EventReq) error {
	callback := &larkservice.CallbackData{}
	err := json.Unmarshal([]byte(event.Body), callback)
	if err != nil {
		log.Errorf("unmarshal callback data failed: %v", err)
		return errors.Wrap(err, "unmarshal")
	}

	log.Debugf("[LARK SSE EVENT IN, data: =====\n%s\n=====", string(callback.Event))

	eventBody := larkservice.ApprovalTaskEvent{}
	err = json.Unmarshal(callback.Event, &eventBody)
	if err != nil {
		log.Errorf("unmarshal callback event failed: %v", err)
		return errors.Wrap(err, "unmarshal")
	}
	log.Infof("LarkEventHandler: new request approval ID %s, request UUID %s, ts: %s", eventBody.AppID, callback.UUID, callback.Ts)
	manager := larkservice.GetLarkApprovalInstanceManager(eventBody.InstanceCode)
	if !manager.CheckAndUpdateUUID(callback.UUID) {
		log.Infof("check existed request uuid %s, ignored", callback.UUID)
		return nil
	}
	t, err := strconv.ParseInt(eventBody.OperateTime, 10, 64)
	if err != nil {
		log.Warnf("parse operate time %s failed: %v", eventBody.OperateTime, err)
	}
	larkservice.UpdateNodeUserApprovalResult(eventBody.InstanceCode, eventBody.DefKey, eventBody.CustomKey, eventBody.OpenID, &larkservice.UserApprovalResult{
		Result:        eventBody.Status,
		OperationTime: t / 1000,
	})
	log.Infof("update lark app info id: %s, instance code: %s, nodeKey: %s, userID: %s status: %s",
		eventBody.AppID,
		eventBody.InstanceCode,
		eventBody.CustomKey,
		eventBody.OpenID,
		eventBody.Status,
	)

	return nil
}
