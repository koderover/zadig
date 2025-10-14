package nacos

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"context"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	mse20190531 "github.com/alibabacloud-go/mse-20190531/v5/client"
	teautil "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/aliyun/credentials-go/credentials"
	credential "github.com/aliyun/credentials-go/credentials"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/koderover/zadig/v2/pkg/types"
	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/util/sets"
)

type MSENacosClient struct {
	*mse20190531.Client
	InstanceID string
}

func NewMSENacosClient(endpoint, accessKeyId, accessKeySecret, instanceID string) (*MSENacosClient, error) {
	credentialConfig := new(credentials.Config).
		SetType("access_key").
		SetAccessKeyId(accessKeyId).
		SetAccessKeySecret(accessKeySecret)
	credential, _err := credential.NewCredential(credentialConfig)
	if _err != nil {
		return nil, _err
	}

	config := &openapi.Config{
		Credential: credential,
	}
	// Endpoint 请参考 https://api.aliyun.com/product/mse
	config.Endpoint = tea.String(endpoint)
	client, _err := mse20190531.NewClient(config)
	if _err != nil {
		return nil, _err
	}
	return &MSENacosClient{Client: client, InstanceID: instanceID}, nil
}

func (c *MSENacosClient) ListGroups(namespaceID, keyword string) ([]*types.NacosDataID, error) {
	namespaceID = getNamespaceID(namespaceID)
	listNacosConfigsRequest := &mse20190531.ListNacosConfigsRequest{
		InstanceId:  tea.String(c.InstanceID),
		NamespaceId: tea.String(namespaceID),
		Group:       tea.String(keyword),
		PageSize:    tea.Int32(999),
		PageNum:     tea.Int32(1),
	}

	runtime := &teautil.RuntimeOptions{}
	resp, err := c.ListNacosConfigsWithOptions(listNacosConfigsRequest, runtime)
	if err != nil {
		err = handleMSEError(err)
		return nil, err
	}

	groupSet := sets.NewString()
	configs := []*types.NacosDataID{}
	for _, config := range resp.Body.Configurations {
		if groupSet.Has(tea.StringValue(config.Group)) {
			continue
		}

		configs = append(configs, &types.NacosDataID{
			Group: tea.StringValue(config.Group),
		})

		groupSet.Insert(tea.StringValue(config.Group))
	}
	return configs, nil
}

func (c *MSENacosClient) ListConfigs(namespaceID, groupName string) ([]*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	listNacosConfigsRequest := &mse20190531.ListNacosConfigsRequest{
		InstanceId:  tea.String(c.InstanceID),
		NamespaceId: tea.String(namespaceID),
		Group:       tea.String(groupName),
		PageSize:    tea.Int32(999),
		PageNum:     tea.Int32(1),
	}

	runtime := &teautil.RuntimeOptions{}
	resp, err := c.ListNacosConfigsWithOptions(listNacosConfigsRequest, runtime)
	if err != nil {
		err = handleMSEError(err)
		return nil, err
	}

	configs := []*types.NacosConfig{}
	for _, config := range resp.Body.Configurations {
		configs = append(configs, &types.NacosConfig{
			NacosDataID: types.NacosDataID{
				DataID: tea.StringValue(config.DataId),
				Group:  tea.StringValue(config.Group),
			},
			NamespaceID: namespaceID,
		})
	}
	return configs, nil
}

func (c *MSENacosClient) GetConfig(dataID, group, namespaceID string) (*types.NacosConfig, error) {
	namespaceID = getNamespaceID(namespaceID)
	runtime := &teautil.RuntimeOptions{}
	getNacosConfigRequest := &mse20190531.GetNacosConfigRequest{
		InstanceId:  tea.String(c.InstanceID),
		NamespaceId: tea.String(namespaceID),
		DataId:      tea.String(dataID),
		Group:       tea.String(group),
	}
	configResp, err := c.GetNacosConfigWithOptions(getNacosConfigRequest, runtime)
	if err != nil {
		return nil, handleMSEError(err)
	}

	return &types.NacosConfig{
		NacosDataID: types.NacosDataID{
			DataID: dataID,
			Group:  group,
		},
		Format:  getFormat(tea.StringValue(configResp.Body.Configuration.Type)),
		Content: tea.StringValue(configResp.Body.Configuration.Content),
	}, nil
}

func (c *MSENacosClient) UpdateConfig(dataID, group, namespaceID, content, format string) error {
	namespaceID = getNamespaceID(namespaceID)
	runtime := &teautil.RuntimeOptions{}
	updateNacosConfigRequest := &mse20190531.UpdateNacosConfigRequest{
		InstanceId:  tea.String(c.InstanceID),
		NamespaceId: tea.String(namespaceID),
		DataId:      tea.String(dataID),
		Group:       tea.String(group),
		Content:     tea.String(content),
		Type:        tea.String(format),
	}
	_, err := c.UpdateNacosConfigWithOptions(updateNacosConfigRequest, runtime)
	if err != nil {
		return handleMSEError(err)
	}

	return nil
}

func (c *MSENacosClient) ListNamespaces() ([]*types.NacosNamespace, error) {
	request := &mse20190531.ListEngineNamespacesRequest{
		InstanceId: tea.String(c.InstanceID),
	}
	runtime := &teautil.RuntimeOptions{}
	mseResp, err := c.ListEngineNamespacesWithOptions(request, runtime)
	if err != nil {
		return nil, handleMSEError(err)
	}

	resp := []*types.NacosNamespace{}
	for _, data := range mseResp.Body.Data {
		resp = append(resp, &types.NacosNamespace{
			NamespaceID:    setNamespaceID(tea.StringValue(data.Namespace)),
			NamespacedName: tea.StringValue(data.NamespaceShowName),
		})
	}
	return resp, nil
}

func (c *MSENacosClient) GetConfigHistory(dataID, group, namespaceID string) ([]*types.NacosConfigHistory, error) {
	namespaceID = getNamespaceID(namespaceID)
	runtime := &teautil.RuntimeOptions{}

	listNacosConfigsRequest := &mse20190531.ListNacosConfigsRequest{
		InstanceId:  tea.String(c.InstanceID),
		NamespaceId: tea.String(namespaceID),
		PageSize:    tea.Int32(999),
		PageNum:     tea.Int32(1),
	}

	resp, err := c.ListNacosConfigsWithOptions(listNacosConfigsRequest, runtime)
	if err != nil {
		err = handleMSEError(err)
		return nil, err
	}

	configs := []*types.NacosConfigHistory{}
	configMap := make(map[string]*types.NacosConfigHistory)
	var mu sync.Mutex

	g, ctx := errgroup.WithContext(context.Background())
	for _, config := range resp.Body.Configurations {
		config := config // 闭包变量

		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			getNacosConfigHistoryRequest := &mse20190531.GetNacosHistoryConfigRequest{
				InstanceId:  tea.String(c.InstanceID),
				NamespaceId: tea.String(namespaceID),
				DataId:      tea.String(tea.StringValue(config.DataId)),
				Group:       tea.String(tea.StringValue(config.Group)),
			}
			configResp, err := c.GetNacosHistoryConfigWithOptions(getNacosConfigHistoryRequest, runtime)
			if err != nil {
				return handleMSEError(err)
			}

			mu.Lock()
			configMap[tea.StringValue(config.DataId)] = &types.NacosConfigHistory{
				AppName: tea.StringValue(configResp.Body.Configuration.AppName),
				Content: tea.StringValue(configResp.Body.Configuration.Content),
				DataID:  tea.StringValue(configResp.Body.Configuration.DataId),
				Group:   tea.StringValue(configResp.Body.Configuration.Group),
				OpType:  tea.StringValue(configResp.Body.Configuration.OpType),
				MD5:     tea.StringValue(configResp.Body.Configuration.Md5),
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, config := range configMap {
		configs = append(configs, config)
	}

	return configs, nil
}

func (c *MSENacosClient) Validate() error {
	req := &mse20190531.GetImportFileUrlRequest{
		InstanceId: tea.String(c.InstanceID),
	}
	_, err := c.GetImportFileUrl(req)
	if err != nil {
		return handleMSEError(err)
	}

	return nil
}

func handleMSEError(err error) error {
	var error = &tea.SDKError{}
	if _t, ok := err.(*tea.SDKError); ok {
		error = _t
	} else {
		error.Message = tea.String(err.Error())
	}

	respErr := fmt.Errorf("error: %s", tea.StringValue(error.Message))

	var data interface{}
	d := json.NewDecoder(strings.NewReader(tea.StringValue(error.Data)))
	d.Decode(&data)
	if m, ok := data.(map[string]interface{}); ok {
		recommend, _ := m["Recommend"]
		respErr = fmt.Errorf("%s, recommend: %s", respErr, recommend)
	}

	log.Error(err)
	return respErr
}
