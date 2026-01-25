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

package cloudservice

import (
	"encoding/json"
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	dms "github.com/alibabacloud-go/dms-enterprise-20181101/v3/client"
	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"
	credential "github.com/aliyun/credentials-go/credentials"
	"github.com/pkg/errors"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

type CloudServiceClient interface {
	Validate() error
}

func ListCloudService(ctx *internalhandler.Context, encryptedKey string) ([]*commonmodels.CloudService, error) {
	aesKey, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, ctx.Logger)
	if err != nil {
		err = fmt.Errorf("ListCloudService GetAesKeyFromEncryptedKey err:%v", err)
		ctx.Logger.Errorf(err.Error())
		return nil, err
	}
	resp, err := commonrepo.NewCloudServiceColl().List(ctx)
	if err != nil {
		err = fmt.Errorf("ListCloudService List err:%v", err)
		ctx.Logger.Errorf(err.Error())
		return nil, err
	}
	for _, cloud := range resp {
		cloud.AccessKeySecret, err = crypto.AesEncryptByKey(cloud.AccessKeySecret, aesKey.PlainText)
		if err != nil {
			err = fmt.Errorf("ListCloudService AesEncryptByKey err:%v", err)
			ctx.Logger.Errorf(err.Error())
			return nil, err
		}
	}
	return resp, nil
}

func ListCloudServiceInfo(ctx *internalhandler.Context) ([]*commonmodels.CloudService, error) {
	resp, err := commonrepo.NewCloudServiceColl().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, cloud := range resp {
		cloud.AccessKeySecret = ""
	}
	return resp, nil
}

func CreateCloudService(ctx *internalhandler.Context, args *commonmodels.CloudService) error {
	if args == nil {
		return errors.New("nil sae")
	}

	if err := commonrepo.NewCloudServiceColl().Create(ctx, args); err != nil {
		err = fmt.Errorf("CreateCloudService err:%v", err)
		ctx.Logger.Errorf(err.Error())
		return err
	}
	return nil
}

func FindCloudService(ctx *internalhandler.Context, id string) (*commonmodels.CloudService, error) {
	return commonrepo.NewCloudServiceColl().Find(ctx, &commonrepo.CloudServiceCollFindOption{Id: id})
}

func UpdateCloudService(ctx *internalhandler.Context, id string, args *commonmodels.CloudService) error {
	return commonrepo.NewCloudServiceColl().Update(ctx, id, args)
}

func DeleteCloudService(ctx *internalhandler.Context, id string) error {
	return commonrepo.NewCloudServiceColl().Delete(ctx, id)
}

func ValidateCloudService(ctx *internalhandler.Context, args *commonmodels.CloudService) error {
	if args == nil {
		return errors.New("nil CloudService")
	}
	switch args.Type {
	case setting.CloudServiceTypeDMS:
		return validateDMS(args)
	case setting.CloudServiceTypeSAE:
		return validateSAE(args)
	default:
		return errors.New("invalid CloudService type")
	}
}

func validateDMS(args *commonmodels.CloudService) error {
	args.Region = "cn-hangzhou"
	client, err := NewDMSClient(args)
	if err != nil {
		return fmt.Errorf("new DMS client err:%v", err)
	}

	listOrdersRequest := &dms.ListOrdersRequest{
		PageNumber: tea.Int32(1),
		PageSize:   tea.Int32(10),
		PluginType: tea.String("DATA_CORRECT"),
	}
	_, err = client.ListOrders(listOrdersRequest)
	if err != nil {
		return fmt.Errorf("validate DMS client err:%v", err)
	}

	return nil
}

func NewDMSClient(dmsSvc *models.CloudService) (*dms.Client, error) {
	credConfig := new(credential.Config).SetType("access_key").SetAccessKeyId(dmsSvc.AccessKeyId).SetAccessKeySecret(dmsSvc.AccessKeySecret)
	credential, err := credential.NewCredential(credConfig)
	if err != nil {
		return nil, err
	}

	config := &openapi.Config{
		Credential: credential,
		Endpoint:   tea.String(fmt.Sprintf("dms-enterprise.%s.aliyuncs.com", dmsSvc.Region)),
	}

	return dms.NewClient(config)
}

func NewSAEClient(saeModel *models.CloudService, regionID string) (result *sae.Client, err error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(saeModel.AccessKeyId),
		AccessKeySecret: tea.String(saeModel.AccessKeySecret),
	}

	config.Endpoint = tea.String(fmt.Sprintf("sae.%s.aliyuncs.com", regionID))
	result, err = sae.NewClient(config)
	return result, err
}

func validateSAE(args *commonmodels.CloudService) error {
	client, err := NewSAEClient(args, "cn-hangzhou")
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
