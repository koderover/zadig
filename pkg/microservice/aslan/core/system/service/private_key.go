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

package service

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	gossh "golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/pm"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/koderover/zadig/v2/pkg/util"
)

func ListPrivateKeys(encryptedKey, projectName, keyword string, systemOnly bool, log *zap.SugaredLogger) ([]*commonmodels.PrivateKey, error) {
	var resp []*commonmodels.PrivateKey
	var err error
	privateKeys, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{ProjectName: projectName, SystemOnly: systemOnly})
	if err != nil {
		log.Errorf("PrivateKey.List error: %s", err)
		return resp, e.ErrListPrivateKeys
	}

	if keyword == "" {
		resp = privateKeys
	} else {
		for _, privateKey := range privateKeys {
			if strings.Contains(privateKey.Name, keyword) || strings.Contains(privateKey.IP, keyword) {
				resp = append(resp, privateKey)
			}
		}
	}

	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		return nil, err
	}
	for _, key := range resp {
		if key.Probe == nil {
			key.Probe = &types.Probe{ProbeScheme: setting.ProtocolTCP}
		}
		key.PrivateKey, err = crypto.AesEncryptByKey(key.PrivateKey, aesKey.PlainText)
		if err != nil {
			return nil, err
		}
	}
	return resp, nil
}

func ListPrivateKeysInternal(log *zap.SugaredLogger) ([]*commonmodels.PrivateKey, error) {
	resp, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		if commonrepo.IsErrNoDocuments(err) {
			return []*commonmodels.PrivateKey{}, nil
		}
		log.Errorf("PrivateKey.List error: %v", err)
		return resp, e.ErrListPrivateKeys
	}
	return resp, nil
}

func GetPrivateKey(id string, log *zap.SugaredLogger) (*commonmodels.PrivateKey, error) {
	resp, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{
		ID: id,
	})
	if err != nil {
		log.Errorf("PrivateKey.Find %s error: %s", id, err)
		return resp, e.ErrGetPrivateKey
	}
	return resp, nil
}

type CreatePrivateKeyResp struct {
	VmID string `json:"vm_id"`
}

func CreatePrivateKey(args *commonmodels.PrivateKey, log *zap.SugaredLogger) (*CreatePrivateKeyResp, error) {
	if !config.CVMNameRegex.MatchString(args.Name) {
		return nil, e.ErrCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
	}

	privateKeyArgs := &commonrepo.PrivateKeyArgs{
		Name: args.Name,
	}

	if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(privateKeyArgs); len(privateKeys) > 0 {
		return nil, e.ErrCreatePrivateKey.AddDesc("Name already exists")
	}

	if args.Agent == nil {
		args.Agent = &commonmodels.VMAgent{}
	} else {
		args.Type = setting.NewVMType
	}
	args.Status = setting.VMCreated

	if args.IP != "" && !util.IsValidIPv4(args.IP) {
		return nil, e.ErrCreatePrivateKey.AddDesc("IP is invalid")
	}

	err := commonrepo.NewPrivateKeyColl().Create(args)
	if err != nil {
		log.Errorf("failed to create privateKey, error: %s", err)
		return nil, e.ErrCreatePrivateKey
	}
	return &CreatePrivateKeyResp{
		VmID: args.ID.Hex(),
	}, nil
}

func UpdatePrivateKey(id string, args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	vm, err := commonrepo.NewPrivateKeyColl().Find(commonrepo.FindPrivateKeyOption{ID: id})
	if err != nil {
		log.Errorf("failed to find privateKey with id: %s, error: %s", id, err)
		return e.ErrUpdatePrivateKey.AddErr(fmt.Errorf("failed to find privateKey with id: %s, err: %s", id, err))
	}

	if args.IP != "" && !util.IsValidIPv4(args.IP) {
		return e.ErrUpdatePrivateKey.AddDesc("IP is invalid")
	}

	if vm.Agent != nil {
		vm.Agent.TaskConcurrency = args.Agent.TaskConcurrency
		vm.Agent.Workspace = args.Agent.Workspace
		args.Agent = vm.Agent
		args.Type = setting.NewVMType
	}

	err = commonrepo.NewPrivateKeyColl().Update(id, args)
	if err != nil {
		log.Errorf("failed to update privateKey, error: %s", err)
		return e.ErrUpdatePrivateKey.AddErr(err)
	}
	return nil
}

func ValidatePrivateKey(args *commonmodels.PrivateKey, log *zap.SugaredLogger) error {
	if args.ScheduleWorkflow {
		return fmt.Errorf("validate only support ssh type")
	}

	privateKey, err := base64.StdEncoding.DecodeString(args.PrivateKey)
	if err != nil {
		return fmt.Errorf("SSH密钥格式错误: %v", err)
	}

	signer, err := gossh.ParsePrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("SSH密钥格式错误: %v", err)
	}

	config := &gossh.ClientConfig{
		User: args.UserName,
		Auth: []gossh.AuthMethod{
			gossh.PublicKeys(signer),
		},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	// 尝试建立 SSH 连接
	addr := fmt.Sprintf("%s:%d", args.IP, args.Port)
	client, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		if strings.Contains(err.Error(), "ssh: handshake failed") {
			return fmt.Errorf("SSH连接失败，请确认：\n1. SSH密钥是否正确\n2. 服务器是否允许SSH连接")
		}
		return fmt.Errorf("SSH连接失败: %v", err)
	}
	defer client.Close()

	// 如果能成功建立连接并创建会话，说明 SSH 密钥是有效的
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("SSH连接验证失败: %v", err)
	}
	session.Close()

	return nil
}

func DeletePrivateKey(id, userName string, log *zap.SugaredLogger) error {
	// 检查该私钥是否被引用
	buildOpt := &commonrepo.BuildListOption{PrivateKeyID: id}
	builds, err := commonrepo.NewBuildColl().List(buildOpt)
	if err == nil && len(builds) != 0 {
		log.Errorf("PrivateKey has been used by build, private key id:%s, product name:%s, build name:%s", id, builds[0].ProductName, builds[0].Name)
		return e.ErrDeleteUsedPrivateKey
	}

	err = commonrepo.NewPrivateKeyColl().Delete(id)
	if err != nil {
		log.Errorf("PrivateKey.Delete %s error: %s", id, err)
		return e.ErrDeletePrivateKey
	}
	// update releated services , which contains the privateKey
	services, err := commonrepo.NewServiceColl().ListMaxRevisions(&commonrepo.ServiceListOption{Type: "pm"})
	if err != nil {
		return err
	}
	for _, service := range services {
		hostIDsSet := sets.NewString()
		for _, config := range service.EnvConfigs {
			hostIDsSet.Insert(config.HostIDs...)
		}
		if !hostIDsSet.Has(id) {
			continue
		}
		// has related hostID
		envConfigs := []*commonmodels.EnvConfig{}
		for _, config := range service.EnvConfigs {
			hostIdsSet := sets.NewString(config.HostIDs...)
			if hostIdsSet.Has(id) {
				hostIdsSet.Delete(id)
				config.HostIDs = hostIdsSet.List()
			}
			envConfigs = append(envConfigs, config)
		}

		envStatus, err := pm.GenerateEnvStatus(service.EnvConfigs, log)
		if err != nil {
			log.Errorf("GenerateEnvStatus err:%s", err)
			continue
		}
		args := &commonservice.ServiceTmplBuildObject{
			ServiceTmplObject: &commonservice.ServiceTmplObject{
				ProductName:  service.ProductName,
				ServiceName:  service.ServiceName,
				Visibility:   service.Visibility,
				Revision:     service.Revision,
				Type:         service.Type,
				Username:     userName,
				HealthChecks: service.HealthChecks,
				EnvConfigs:   envConfigs,
				EnvStatuses:  envStatus,
				From:         "deletePriveteKey",
			},
			Build: &commonmodels.Build{Name: service.BuildName},
		}
		if err := commonservice.UpdatePmServiceTemplate(userName, args, log); err != nil {
			log.Errorf("UpdatePmServiceTemplate err :%s", err)
			continue
		}
	}
	return nil
}

func ListLabels() ([]string, error) {
	vms, err := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{})
	if err != nil {
		return nil, fmt.Errorf("failed to list vms: %v", err)
	}
	resp := make([]string, 0)
	for _, vm := range vms {
		if vm.Agent == nil {
			resp = append(resp, vm.Label)
		}
	}
	return resp, nil
}

// override: Full coverage (temporarily reserved)
// increment: Incremental coverage
// patch: Overwrite existing
func BatchCreatePrivateKey(args []*commonmodels.PrivateKey, option, username string, log *zap.SugaredLogger) error {
	switch option {
	case "increment":
		for _, currentPrivateKey := range args {
			if !config.CVMNameRegex.MatchString(currentPrivateKey.Name) {
				return e.ErrBulkCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
			}

			if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{Name: currentPrivateKey.Name}); len(privateKeys) > 0 {
				continue
			}

			currentPrivateKey.UpdateBy = username
			if err := commonrepo.NewPrivateKeyColl().Create(currentPrivateKey); err != nil {
				log.Errorf("PrivateKey.Create error: %s", err)
				return e.ErrBulkCreatePrivateKey.AddDesc("bulk add privateKey failed")
			}
		}

	case "patch":
		for _, currentPrivateKey := range args {
			if !config.CVMNameRegex.MatchString(currentPrivateKey.Name) {
				return e.ErrBulkCreatePrivateKey.AddDesc("主机名称仅支持字母，数字和下划线且首个字符不以数字开头")
			}
			currentPrivateKey.UpdateBy = username
			if privateKeys, _ := commonrepo.NewPrivateKeyColl().List(&commonrepo.PrivateKeyArgs{Name: currentPrivateKey.Name}); len(privateKeys) > 0 {
				if err := commonrepo.NewPrivateKeyColl().Update(privateKeys[0].ID.Hex(), currentPrivateKey); err != nil {
					log.Errorf("PrivateKey.update error: %s", err)
					return e.ErrBulkCreatePrivateKey.AddDesc("bulk update privateKey failed")
				}
				continue
			}
			if err := commonrepo.NewPrivateKeyColl().Create(currentPrivateKey); err != nil {
				log.Errorf("PrivateKey.Create error: %s", err)
				return e.ErrBulkCreatePrivateKey.AddDesc("bulk add privateKey failed")
			}
		}
	}

	return nil
}
