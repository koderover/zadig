package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"

	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/setting"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func ListConfigurationManagement(_type string, log *zap.SugaredLogger) ([]*commonmodels.ConfigurationManagement, error) {
	resp, err := mongodb.NewConfigurationManagementColl().List(context.Background(), _type)
	if err != nil {
		log.Errorf("list configuration management error: %v", err)
		return nil, e.ErrListConfigurationManagement
	}

	return resp, nil
}

func CreateConfigurationManagement(args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	if err := validateConfigurationManagementType(args); err != nil {
		return e.ErrCreateConfigurationManagement.AddErr(err)
	}
	if err := marshalConfigurationManagementAuthConfig(args); err != nil {
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

	return resp, nil
}

func UpdateConfigurationManagement(id string, args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	if err := validateConfigurationManagementType(args); err != nil {
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}
	if err := marshalConfigurationManagementAuthConfig(args); err != nil {
		log.Errorf("marshal configuration management error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}

	err := mongodb.NewConfigurationManagementColl().Update(context.Background(), id, args)
	if err != nil {
		log.Errorf("update configuration management error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddErr(err)
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
		return validateNacosAuthConfig(getNacosConfigFromRaw(rawData))
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
		return e.ErrValidateConfigurationManagement.AddDesc(fmt.Sprintf("unexpected HTTP status code %d when connecting to apollo", resp.StatusCode))
	}
	return nil
}

func validateNacosAuthConfig(config *commonmodels.NacosConfig) error {
	u, err := url.Parse(config.ServerAddress)
	if err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	if u.Path == "" {
		u.Path = "/nacos"
	}
	u = u.JoinPath("v1/auth/login")

	resp, err := req.R().AddQueryParam("username", config.UserName).
		AddQueryParam("password", config.Password).
		Post(u.String())
	if err != nil {
		return e.ErrValidateConfigurationManagement.AddErr(err)
	}
	if resp.StatusCode != http.StatusOK {
		return e.ErrValidateConfigurationManagement.AddDesc(fmt.Sprintf("unexpected HTTP status code %d when connecting to nacos", resp.StatusCode))
	}
	return nil
}

func getApolloConfigFromRaw(raw string) *commonmodels.ApolloConfig {
	return &commonmodels.ApolloConfig{
		ServerAddress: gjson.Get(raw, "server_address").String(),
		ApolloAuthConfig: &commonmodels.ApolloAuthConfig{
			Token: gjson.Get(raw, "auth_config.token").String(),
		},
	}
}

func getNacosConfigFromRaw(raw string) *commonmodels.NacosConfig {
	return &commonmodels.NacosConfig{
		ServerAddress: gjson.Get(raw, "server_address").String(),
		NacosAuthConfig: &commonmodels.NacosAuthConfig{
			UserName: gjson.Get(raw, "auth_config.user_name").String(),
			Password: gjson.Get(raw, "auth_config.password").String(),
		},
	}
}

func marshalConfigurationManagementAuthConfig(management *commonmodels.ConfigurationManagement) error {
	rawData, err := json.Marshal(management.AuthConfig)
	if err != nil {
		return err
	}

	rawJson := string(rawData)
	switch management.Type {
	case setting.SourceFromApollo:
		management.AuthConfig = &commonmodels.ApolloAuthConfig{
			Token: gjson.Get(rawJson, "token").String(),
		}
	case setting.SourceFromNacos:
		management.AuthConfig = &commonmodels.NacosAuthConfig{
			UserName: gjson.Get(rawJson, "user_name").String(),
			Password: gjson.Get(rawJson, "password").String(),
		}
	default:
		return errors.New("marshal auth config: invalid type")
	}
	return nil
}

func validateConfigurationManagementType(management *commonmodels.ConfigurationManagement) error {
	if management.Type != setting.SourceFromApollo && management.Type != setting.SourceFromNacos {
		return errors.New("invalid type")
	}
	return nil
}
