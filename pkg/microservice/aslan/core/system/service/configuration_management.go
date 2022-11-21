package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/imroc/req/v3"
	"github.com/pkg/errors"
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
	return resp, nil
}

func CreateConfigurationManagement(args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	err := mongodb.NewConfigurationManagementColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create configuration management error: %v", err)
		return e.ErrCreateConfigurationManagement.AddDesc(err.Error())
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
		return e.ErrDeleteConfigurationManagement.AddDesc(err.Error())
	}
	return nil
}

func ValidateConfigurationManagement(args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	switch args.Type {
	case setting.SourceFromApollo:
		config, err := UnmarshalApolloAuthConfig(args.AuthConfig)
		if err != nil {
			return e.ErrInvalidParam.AddErr(errors.Wrap(err, "unmarshal apollo auth config"))
		}
		return validateApolloAuthConfig(args.ServerAddress, config)
	case setting.SourceFromNacos:
		return nil
	default:
		return e.ErrInvalidParam.AddDesc("invalid type")
	}
}

func UnmarshalApolloAuthConfig(raw string) (*commonmodels.ApolloAuthConfig, error) {
	var config *commonmodels.ApolloAuthConfig
	return config, json.Unmarshal([]byte(raw), config)
}

func validateApolloAuthConfig(serverAddr string, config *commonmodels.ApolloAuthConfig) error {
	u, err := url.Parse(serverAddr)
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
		return e.ErrValidateConfigurationManagement.AddDesc(fmt.Sprintf("bad HTTP status code %d", resp.StatusCode))
	}
	return nil
}
