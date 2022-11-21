package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListConfigurationManagement(log *zap.SugaredLogger) ([]*commonmodels.ConfigurationManagement, error) {
	resp, err := mongodb.NewConfigurationManagementColl().List(context.Background())
	if err != nil {
		log.Errorf("list configuration management error: %v", err)
		return nil, e.ErrListConfigurationManagement
	}
	for _, management := range resp {
		if err := unmarshalAuthConfig(management); err != nil {
			log.Errorf("unmarshal configuration management error: %v", err)
			return nil, e.ErrListConfigurationManagement.AddErr(err)
		}
	}
	return resp, nil
}

func CreateConfigurationManagement(args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	if err := marshalAuthConfig(args); err != nil {
		log.Errorf("marshal configuration management error: %v", err)
		return e.ErrCreateConfigurationManagement.AddErr(err)
	}

	err := mongodb.NewConfigurationManagementColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create configuration management error: %v", err)
		return e.ErrCreateConfigurationManagement.AddErr(err)
	}
	return nil
}

func GetConfigurationManagement(id string, log *zap.SugaredLogger) (*commonmodels.ConfigurationManagement, error) {
	resp, err := mongodb.NewConfigurationManagementColl().GetByID(context.Background(), id)
	if err != nil {
		log.Errorf("get configuration management error: %v", err)
		return nil, e.ErrGetConfigurationManagement.AddErr(err)
	}
	if err = unmarshalAuthConfig(resp); err != nil {
		log.Errorf("unmarshal configuration management error: %v", err)
		return nil, e.ErrGetConfigurationManagement.AddErr(err)
	}
	return resp, nil
}

func UpdateConfigurationManagement(id string, args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	if err := marshalAuthConfig(args); err != nil {
		log.Errorf("marshal configuration management error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}
	err := mongodb.NewConfigurationManagementColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update configuration management error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddDesc(err.Error())
	}
	return nil
}

func DeleteConfigurationManagement(id string, log *zap.SugaredLogger) error {
	err := mongodb.NewConfigurationManagementColl().DeleteByID(context.Background(), id)
	if err != nil {
		log.Errorf("delete configuration management error: %v", err)
		return e.ErrDeleteConfigurationManagement.AddErr(err)
	}
	return nil
}

func ValidateConfigurationManagement(rawData string, log *zap.SugaredLogger) error {
	switch gjson.Get(rawData, "type").String() {
	case setting.SourceFromApollo:
		return validateApolloAuthConfig(getApolloConfigFromRaw(rawData))
	case setting.SourceFromNacos:
		// todo
		return nil
	default:
		return e.ErrInvalidParam.AddDesc("invalid type")
	}
}

func validateApolloAuthConfig(config *commonmodels.ApolloConfig) error {
	u, err := url.Parse(config.ServerAddress)
	if err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	u = u.JoinPath("/openapi/v1/apps")

	resp, err := req.R().SetHeaders(map[string]string{"Authorization": config.Token}).
		SetContentType("application/json;charset=UTF-8").
		Get(u.String())

	if err != nil {
		return e.ErrValidateConfigurationManagement.AddErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		return e.ErrValidateConfigurationManagement.AddDesc(fmt.Sprintf("unexpected HTTP status code %d", resp.StatusCode))
	}
	return nil
}

func GetApolloConfig(serverAddr string, authConfigString string) *commonmodels.ApolloConfig {
	return &commonmodels.ApolloConfig{
		ServerAddress: serverAddr,
		Token:         gjson.Get(authConfigString, "token").String(),
	}
}

func GetNacosConfig(serverAddr string, authConfigString string) *commonmodels.NacosConfig {
	return &commonmodels.NacosConfig{
		ServerAddress: serverAddr,
		UserName:      gjson.Get(authConfigString, "user_name").String(),
		Password:      gjson.Get(authConfigString, "password").String(),
	}
}

func getApolloConfigFromRaw(raw string) *commonmodels.ApolloConfig {
	return &commonmodels.ApolloConfig{
		ServerAddress: gjson.Get(raw, "server_addr").String(),
		Token:         gjson.Get(raw, "auth_config.token").String(),
	}
}

func getNacosConfigFromRaw(raw string) *commonmodels.NacosConfig {
	return &commonmodels.NacosConfig{
		ServerAddress: gjson.Get(raw, "server_addr").String(),
		UserName:      gjson.Get(raw, "auth_config.user_name").String(),
		Password:      gjson.Get(raw, "auth_config.password").String(),
	}
}

func marshalAuthConfig(management *commonmodels.ConfigurationManagement) error {
	authConfig, err := json.Marshal(management.AuthConfig)
	if err != nil {
		return err
	}
	management.AuthConfigString = string(authConfig)
	return nil
}

func unmarshalAuthConfig(management *commonmodels.ConfigurationManagement) error {
	return json.Unmarshal([]byte(management.AuthConfigString), &management.AuthConfig)
}
