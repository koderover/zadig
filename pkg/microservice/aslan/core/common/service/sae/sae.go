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

package sae

import (
	"fmt"

	"github.com/segmentio/encoding/json"

	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

func ListSAE(encryptedKey string, log *zap.SugaredLogger) ([]*commonmodels.SAE, error) {
	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		log.Errorf("ListSAE GetAesKeyFromEncryptedKey err:%v", err)
		return nil, err
	}
	resp, err := commonrepo.NewSAEColl().List()
	if err != nil {
		return []*commonmodels.SAE{}, nil
	}
	for _, sae := range resp {
		sae.AccessKeySecret, err = crypto.AesEncryptByKey(sae.AccessKeySecret, aesKey.PlainText)
		if err != nil {
			log.Errorf("ListSAE AesEncryptByKey err:%v", err)
			return nil, err
		}
	}
	return resp, nil
}

func ListSAEInfo(log *zap.SugaredLogger) ([]*commonmodels.SAE, error) {
	resp, err := commonrepo.NewSAEColl().List()
	if err != nil {
		return nil, err
	}
	for _, sae := range resp {
		sae.AccessKeySecret = ""
	}
	return resp, nil
}

func CreateSAE(args *commonmodels.SAE, log *zap.SugaredLogger) error {
	if args == nil {
		return errors.New("nil sae")
	}

	if err := commonrepo.NewSAEColl().Create(args); err != nil {
		log.Errorf("CreateSAE err:%v", err)
		return err
	}
	return nil
}

func FindSAE(id, name string) (*commonmodels.SAE, error) {
	return commonrepo.NewSAEColl().Find(&commonrepo.SAECollFindOption{Id: id, Name: name})
}

func UpdateSAE(id string, args *commonmodels.SAE, log *zap.SugaredLogger) error {
	return commonrepo.NewSAEColl().Update(id, args)
}

func DeleteSAE(id string) error {
	return commonrepo.NewSAEColl().Delete(id)
}

func ValidateSAE(args *commonmodels.SAE) error {
	if args == nil {
		return errors.New("nil SAE")
	}
	return validateSAE(args)
}

func validateSAE(args *commonmodels.SAE) error {
	client, err := NewClient(args, "cn-hangzhou")
	if err != nil {
		return fmt.Errorf("new SAE client err:%v", err)
	}

	describeNamespacesRequest := &sae.DescribeNamespacesRequest{}
	saeResp, err := client.DescribeNamespaces(describeNamespacesRequest)
	if err != nil {
		return fmt.Errorf("Failed to describe namespace list err: %v", err)
	}
	if !tea.BoolValue(saeResp.Body.Success) {
		return fmt.Errorf("Failed to describe namespace list, statusCode: %d, code: %s, errCode: %s, message: %s", tea.Int32Value(saeResp.StatusCode), tea.StringValue(saeResp.Body.Code), tea.StringValue(saeResp.Body.ErrorCode), tea.StringValue(saeResp.Body.Message))
	}

	return nil
}

type SAEKV struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *SAEValueFrom `json:"valueFrom,omitempty"`
}

type SAEValueFrom struct {
	ConfigMapRef *SAEConfigMapRef `json:"configMapRef,omitempty"`
}

type SAEConfigMapRef struct {
	ConfigMapID int    `json:"configMapId"`
	Key         string `json:"key"`
}

// CreateKVMap takes a string and de-serialize it into a struct that we can use
func CreateKVMap(kv *string) (map[string]*commonmodels.SAEKV, error) {
	envList := make([]*SAEKV, 0)
	err := json.Unmarshal([]byte(tea.StringValue(kv)), &envList)
	if err != nil {
		return nil, err
	}

	resp := make(map[string]*commonmodels.SAEKV)

	for _, env := range envList {
		resp[env.Name] = &commonmodels.SAEKV{
			Name:  env.Name,
			Value: env.Value,
			//ValueFrom:   env.ValueFrom,

		}

		if env.ValueFrom != nil && env.ValueFrom.ConfigMapRef != nil {
			resp[env.Name].ConfigMapID = env.ValueFrom.ConfigMapRef.ConfigMapID
			resp[env.Name].Key = env.ValueFrom.ConfigMapRef.Key
		}
	}

	return resp, nil
}

func ToSAEKVString(envs []*commonmodels.SAEKV) (*string, error) {
	if len(envs) == 0 {
		return nil, nil
	}
	resp := make([]*SAEKV, 0)
	for _, kv := range envs {
		saekv := &SAEKV{
			Name:  kv.Name,
			Value: kv.Value,
		}

		if kv.ConfigMapID != 0 {
			saekv.ValueFrom = &SAEValueFrom{ConfigMapRef: &SAEConfigMapRef{
				ConfigMapID: kv.ConfigMapID,
				Key:         kv.Key,
			}}
		}
		resp = append(resp, saekv)
	}

	envStr, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}

	return tea.String(string(envStr)), nil
}
