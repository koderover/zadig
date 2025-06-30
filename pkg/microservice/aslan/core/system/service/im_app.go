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
	"fmt"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	// larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/dingtalk"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/lark"
	"github.com/koderover/zadig/v2/pkg/tool/workwx"
)

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
	case setting.IMLark:
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
	err := lark.Validate(args.AppID, args.AppSecret)
	if err != nil {
		return e.ErrCreateIMApp.AddErr(errors.Wrap(err, "validate"))
	}

	_, err = mongodb.NewIMAppColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create lark IM error: %v", err)
		return e.ErrCreateIMApp.AddErr(err)
	}

	err = CreateLarkSSEConnection(args)
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
	case setting.IMLark:
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
	err := lark.Validate(args.AppID, args.AppSecret)
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
	case setting.IMLark:
		return lark.Validate(im.AppID, im.AppSecret)
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

func CreateLarkSSEConnection(arg *commonmodels.IMApp) error {
	if arg.Type != setting.IMLark {
		return fmt.Errorf("invalid type: %s to create lark sse connection", arg.Type)
	}

	// nothing to create
	if arg.LarkEventType != setting.LarkEventTypeSSE {
		return nil
	}

	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnCustomizedEvent("approval_task", func(ctx context.Context, event *larkevent.EventReq) error {
                        fmt.Printf("[ OnCustomizedEvent access ], type: message, data: %s\n", string(event.Body))
                        return nil
                })

	// 创建Client
	cli := larkws.NewClient(arg.AppID, arg.AppSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)
	// 启动客户端
	err := cli.Start(context.Background())
	if err != nil {
		return err
	}
	return nil
}
