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

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

func PresetSystemAdmin(email string, password, domain string) (string, error) {
	r, err := user.New().SearchUser(&user.SearchUserArgs{Account: setting.PresetAccount})
	if err != nil {
		log.Errorf("SearchUser err:%s", err)
		return "", err
	}

	if r.TotalCount > 0 {
		log.Infof("User admin exists, skip it.")
		return r.Users[0].UID, nil
	}
	user, err := user.New().CreateUser(&user.CreateUserArgs{
		Name:     setting.PresetAccount,
		Password: password,
		Account:  setting.PresetAccount,
		Email:    email,
	})
	if err != nil {
		log.Errorf("created  admin err:%s", err)
		return "", err
	}
	// report register
	err = reportRegister(domain, email)
	if err != nil {
		log.Errorf("reportRegister err: %s", err)
	}
	return user.Uid, nil
}

func PresetRoleBinding(uid string) error {
	return policy.NewDefault().CreateOrUpdateSystemRoleBinding(&policy.RoleBinding{
		Name: config.RoleBindingNameFromUIDAndRole(uid, setting.SystemAdmin, "*"),
		UID:  uid,
		Role: string(setting.SystemAdmin),
		Type: setting.ResourceTypeSystem,
	})

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
