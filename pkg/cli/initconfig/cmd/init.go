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

package cmd

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/aslan"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/httpclient"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	rootCmd.AddCommand(initCmd)
	log.Init(&log.Config{
		Level: config.LogLevel(),
	})
}

//go:embed contributor.yaml
var contributor []byte

//go:embed read-only.yaml
var readOnly []byte

//go:embed admin.yaml
var admin []byte

//go:embed project-admin.yaml
var projectAdmin []byte

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "init system config",
	Long:  `init system config.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := run(); err != nil {
			log.Fatal(err)
		}
	},
}

func run() error {
	for {
		err := Healthz()
		if err == nil {
			break
		}
		log.Error(err)
		time.Sleep(10 * time.Second)
	}
	err := initSystemConfig()
	if err == nil {
		log.Info("zadig init success")
	}
	return err
}

func initSystemConfig() error {
	email := config.AdminEmail()
	password := config.AdminPassword()
	domain := config.SystemAddress()

	uid, err := presetSystemAdmin(email, password, domain)
	if err != nil {
		log.Errorf("presetSystemAdmin err:%s", err)
		return err
	}
	if err := presetRole(); err != nil {
		log.Errorf("presetRole err:%s", err)
		return err
	}

	if err := presetRoleBinding(uid); err != nil {
		log.Errorf("presetRoleBinding err:%s", err)
		return err
	}

	if err := createLocalCluster(); err != nil {
		log.Errorf("createLocalCluster err:%s", err)
		return err
	}

	return nil
}

func presetSystemAdmin(email string, password, domain string) (string, error) {
	r, err := user.New().SearchUser(&user.SearchUserArgs{Account: setting.PresetAccount})
	if err != nil {
		log.Errorf("SearchUser err:%s", err)
		return "", err
	}
	if len(r.Users) > 0 {
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

type Operation struct {
	Data string `json:"data"`
}
type Register struct {
	Domain    string `json:"domain"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	CreatedAt int64  `json:"created_at"`
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

func presetRoleBinding(uid string) error {
	return policy.NewDefault().CreateOrUpdateSystemRoleBinding(&policy.RoleBinding{
		Name: fmt.Sprintf(setting.RoleBindingNameFmt, setting.RoleAdmin, setting.PresetAccount, ""),
		UID:  uid,
		Role: setting.RoleAdmin,
	})

}

func presetRole() error {
	g := new(errgroup.Group)
	g.Go(func() error {
		systemRole := &policy.Role{}
		if err := yaml.Unmarshal(admin, systemRole); err != nil {
			log.DPanic(err)
		}
		return policy.NewDefault().CreateSystemRole(systemRole.Name, systemRole)
	})

	rolesArray := [][]byte{readOnly, contributor, projectAdmin}

	for _, v := range rolesArray {
		role := &policy.Role{}
		if err := yaml.Unmarshal(v, role); err != nil {
			log.DPanic(err)
		}
		g.Go(func() error {
			return policy.NewDefault().CreatePublicRole(role.Name, role)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}

func createLocalCluster() error {
	cluster, err := aslan.New(config.AslanServiceAddress()).GetLocalCluster()
	if err != nil {
		return err
	}
	if cluster != nil {
		return nil
	}
	return aslan.New(config.AslanServiceAddress()).AddLocalCluster()
}
