/*
Copyright 2023 The KodeRover Authors.

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

package user

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/policy/core/service"
	userservice "github.com/koderover/zadig/pkg/microservice/user/core/service/user"
	"github.com/koderover/zadig/pkg/setting"
	internalhandler "github.com/koderover/zadig/pkg/shared/handler"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

func PresetSystemAdmin(email string, password, domain string, logger *zap.SugaredLogger) (string, bool, error) {
	exist, err := userservice.CheckUserExist(logger)
	if err != nil {
		log.Errorf("failed to check user exist in db, error:%s", err)
		return "", false, err
	}
	if exist {
		log.Infof("User exists, skip it.")
		return "", false, nil
	}

	userArgs := &userservice.User{
		Name:     setting.PresetAccount,
		Password: password,
		Account:  setting.PresetAccount,
		Email:    email,
	}
	user, err := userservice.CreateUser(userArgs, logger)
	if err != nil {
		log.Errorf("created  admin err:%s", err)
		return "", false, err
	}
	// report register
	err = reportRegister(domain, email)
	if err != nil {
		log.Errorf("reportRegister err: %s", err)
	}
	return user.UID, true, nil
}

func PresetRoleBinding(c *gin.Context, uid string, logger *zap.SugaredLogger) error {
	args := &service.RoleBinding{
		Name:   config.RoleBindingNameFromUIDAndRole(uid, setting.SystemAdmin, "*"),
		UID:    uid,
		Role:   string(setting.SystemAdmin),
		Type:   setting.ResourceTypeSystem,
		Preset: false,
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	detail := "用户：" + setting.PresetAccount + "，角色名称：" + args.Role
	internalhandler.InsertDetailedOperationLog(c, "system", "", setting.OperationSceneSystem, "创建或更新", "系统角色绑定", detail, string(data), logger, args.Name)
	return service.CreateOrUpdateSystemRoleBinding(service.SystemScope, args, logger)
}

type Register struct {
	Domain    string `json:"domain"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
}

type Operation struct {
	Data string `json:"data"`
}

func reportRegister(domain, email string) error {
	register := Register{
		Domain:    domain,
		Username:  "admin",
		Email:     email,
		CreatedAt: time.Now().Unix(),
	}
	registerByte, _ := json.Marshal(register)
	encrypt, err := RSAEncrypt([]byte(registerByte))
	if err != nil {
		log.Errorf("RSAEncrypt err: %s", err)
		return err
	}
	encodeString := base64.StdEncoding.EncodeToString(encrypt)
	reqBody := Operation{Data: encodeString}
	_, err = httpclient.Post("https://api.koderover.com/api/operation/admin/user", httpclient.SetBody(reqBody))
	return err
}
