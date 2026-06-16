package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	commonutil "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/util"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/pkg/errors"

	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/apollo"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
	"github.com/koderover/zadig/v2/pkg/tool/nacos"
)

func ListConfigurationManagement(encryptedKey, _type string, log *zap.SugaredLogger) ([]*commonmodels.ConfigurationManagement, error) {
	var (
		resp []*commonmodels.ConfigurationManagement
		err  error
	)
	if _type == "nacos" {
		resp, err = mongodb.NewConfigurationManagementColl().ListNacos(context.Background())
		if err != nil {
			log.Errorf("list nacos configuration management error: %v", err)
			return nil, e.ErrListConfigurationManagement
		}
	} else {
		resp, err = mongodb.NewConfigurationManagementColl().List(context.Background(), _type)
		if err != nil {
			log.Errorf("list configuration management error: %v", err)
			return nil, e.ErrListConfigurationManagement
		}
	}
	if err := protectConfigurationManagementSecrets(resp, encryptedKey, log); err != nil {
		log.Errorf("encrypt configuration management password error: %v", err)
		return nil, e.ErrListConfigurationManagement.AddErr(err)
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

	if _, err := mongodb.NewConfigurationManagementColl().GetBySystemIdentity(args.SystemIdentity); err == nil {
		return e.ErrCreateConfigurationManagement.AddErr(fmt.Errorf("can't set the same system identity"))
	}
	err := mongodb.NewConfigurationManagementColl().Create(context.Background(), args)
	if err != nil {
		log.Errorf("create configuration management error: %v", err)
		return e.ErrCreateConfigurationManagement.AddErr(err)
	}
	return nil
}

func GetConfigurationManagement(id, encryptedKey string, log *zap.SugaredLogger) (*commonmodels.ConfigurationManagement, error) {
	resp, err := mongodb.NewConfigurationManagementColl().GetByID(context.Background(), id)
	if err != nil {
		log.Errorf("get configuration management error: %v", err)
		return nil, e.ErrGetConfigurationManagement.AddErr(err)
	}
	if err := protectConfigurationManagementSecrets([]*commonmodels.ConfigurationManagement{resp}, encryptedKey, log); err != nil {
		log.Errorf("encrypt configuration management password error: %v", err)
		return nil, e.ErrGetConfigurationManagement.AddErr(err)
	}

	return resp, nil
}

func protectConfigurationManagementSecrets(managements []*commonmodels.ConfigurationManagement, encryptedKey string, log *zap.SugaredLogger) error {
	targets := make([]*nacos.NacosAuthConfig, 0)
	for _, management := range managements {
		if management == nil {
			continue
		}
		if err := marshalConfigurationManagementAuthConfig(management); err != nil {
			return err
		}

		switch authConfig := management.AuthConfig.(type) {
		case *commonmodels.ApolloAuthConfig:
			if authConfig.Token != "" {
				authConfig.Token = setting.MaskValue
			}
		case *nacos.NacosAuthConfig:
			if authConfig.Password == "" {
				continue
			}
			if encryptedKey == "" {
				authConfig.Password = setting.MaskValue
				continue
			}
			targets = append(targets, authConfig)
		case *nacos.NacosEEMSEAuthConfig:
			if authConfig.AccessKeySecret != "" {
				authConfig.AccessKeySecret = setting.MaskValue
			}
		}
	}

	if len(targets) == 0 {
		return nil
	}

	aesKeyResp, err := commonutil.GetAesKeyFromEncryptedKey(encryptedKey, log)
	if err != nil {
		return err
	}

	encryptedPasswords := make([]string, len(targets))
	for i, authConfig := range targets {
		encryptedPasswords[i], err = crypto.AesEncryptByKey(authConfig.Password, aesKeyResp.PlainText)
		if err != nil {
			return err
		}
	}

	for i, authConfig := range targets {
		authConfig.Password = encryptedPasswords[i]
	}

	return nil
}

func UpdateConfigurationManagement(id string, args *commonmodels.ConfigurationManagement, log *zap.SugaredLogger) error {
	if err := validateConfigurationManagementType(args); err != nil {
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}
	if err := marshalConfigurationManagementAuthConfig(args); err != nil {
		log.Errorf("marshal configuration management error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}
	oldCm, err := restoreMaskedConfigurationManagementSecrets(id, args)
	if err != nil {
		log.Errorf("restore configuration management masked secrets error: %v", err)
		return e.ErrUpdateConfigurationManagement.AddErr(err)
	}
	if oldCm == nil {
		oldCm, err = mongodb.NewConfigurationManagementColl().GetByID(context.Background(), id)
		if err != nil {
			log.Errorf("get configuration management error: %v", err)
			return e.ErrUpdateConfigurationManagement.AddErr(err)
		}
	}
	oldSystemIdentity := oldCm.SystemIdentity
	if oldCm.SystemIdentity != "" && args.SystemIdentity != oldSystemIdentity {
		if _, err := mongodb.NewConfigurationManagementColl().GetBySystemIdentity(args.SystemIdentity); err == nil {
			return e.ErrUpdateConfigurationManagement.AddErr(fmt.Errorf("can't set the same system identity"))
		}
	}
	err = mongodb.NewConfigurationManagementColl().Update(context.Background(), id, args)
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
	management := new(commonmodels.ConfigurationManagement)
	if err := json.Unmarshal([]byte(rawData), management); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	if err := validateConfigurationManagementType(management); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	if err := marshalConfigurationManagementAuthConfig(management); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}
	if _, err := restoreMaskedConfigurationManagementSecrets("", management); err != nil {
		return e.ErrValidateConfigurationManagement.AddErr(err)
	}

	switch management.Type {
	case setting.SourceFromApollo:
		return validateApolloAuthConfig(&commonmodels.ApolloConfig{ServerAddress: management.ServerAddress, ApolloAuthConfig: management.AuthConfig.(*commonmodels.ApolloAuthConfig)})
	case setting.SourceFromNacos, setting.SourceFromNacos3:
		return validateNacosAuthConfig(&nacos.NacosConfig{Type: management.Type, ServerAddress: management.ServerAddress, NacosAuthConfig: management.AuthConfig})
	case setting.SourceFromNacosEEMSE:
		return validateNacosAuthConfig(&nacos.NacosConfig{Type: management.Type, ServerAddress: management.ServerAddress, NacosAuthConfig: management.AuthConfig})
	default:
		return e.ErrInvalidParam.AddDesc("invalid type")
	}
}

func restoreMaskedConfigurationManagementSecrets(id string, management *commonmodels.ConfigurationManagement) (*commonmodels.ConfigurationManagement, error) {
	if management == nil {
		return nil, nil
	}

	oldManagement, err := getConfigurationManagementForMaskedSecrets(id, management)
	if err != nil || oldManagement == nil {
		return oldManagement, err
	}
	if err := marshalConfigurationManagementAuthConfig(oldManagement); err != nil {
		return nil, err
	}

	switch authConfig := management.AuthConfig.(type) {
	case *commonmodels.ApolloAuthConfig:
		oldAuthConfig, ok := oldManagement.AuthConfig.(*commonmodels.ApolloAuthConfig)
		if ok && authConfig.Token == setting.MaskValue {
			authConfig.Token = oldAuthConfig.Token
		}
	case *nacos.NacosAuthConfig:
		oldAuthConfig, ok := oldManagement.AuthConfig.(*nacos.NacosAuthConfig)
		if ok && authConfig.Password == setting.MaskValue {
			authConfig.Password = oldAuthConfig.Password
		}
	case *nacos.NacosEEMSEAuthConfig:
		oldAuthConfig, ok := oldManagement.AuthConfig.(*nacos.NacosEEMSEAuthConfig)
		if ok && authConfig.AccessKeySecret == setting.MaskValue {
			authConfig.AccessKeySecret = oldAuthConfig.AccessKeySecret
		}
	}

	return oldManagement, nil
}

func getConfigurationManagementForMaskedSecrets(id string, management *commonmodels.ConfigurationManagement) (*commonmodels.ConfigurationManagement, error) {
	if management == nil {
		return nil, nil
	}

	hasMaskedSecret := false
	switch authConfig := management.AuthConfig.(type) {
	case *commonmodels.ApolloAuthConfig:
		hasMaskedSecret = authConfig.Token == setting.MaskValue
	case *nacos.NacosAuthConfig:
		hasMaskedSecret = authConfig.Password == setting.MaskValue
	case *nacos.NacosEEMSEAuthConfig:
		hasMaskedSecret = authConfig.AccessKeySecret == setting.MaskValue
	}
	if !hasMaskedSecret {
		return nil, nil
	}

	if id != "" {
		return mongodb.NewConfigurationManagementColl().GetByID(context.Background(), id)
	}
	if !management.ID.IsZero() {
		return mongodb.NewConfigurationManagementColl().GetByID(context.Background(), management.ID.Hex())
	}
	if management.SystemIdentity != "" {
		return mongodb.NewConfigurationManagementColl().GetBySystemIdentity(management.SystemIdentity)
	}
	return nil, fmt.Errorf("configuration management id is required when secret fields are masked")
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

func validateNacosAuthConfig(config *nacos.NacosConfig) error {
	if config.Type != setting.SourceFromNacos && config.Type != setting.SourceFromNacos3 && config.Type != setting.SourceFromNacosEEMSE {
		return fmt.Errorf("nacos type is not nacos 1.x or nacos 3.x or nacos ee mse")
	}

	client, err := nacos.NewNacosClient(config.Type, config.ServerAddress, config.NacosAuthConfig)
	if err != nil {
		return e.ErrValidateConfigurationManagement.AddErr(err)
	}

	err = client.Validate()
	if err != nil {
		return e.ErrValidateConfigurationManagement.AddErr(err)
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

func getNacosConfigFromRaw(raw string) *nacos.NacosConfig {
	return &nacos.NacosConfig{
		Type:          setting.SourceFromNacos,
		ServerAddress: gjson.Get(raw, "server_address").String(),
		NacosAuthConfig: &nacos.NacosAuthConfig{
			UserName: gjson.Get(raw, "auth_config.user_name").String(),
			Password: gjson.Get(raw, "auth_config.password").String(),
		},
	}
}

func getNacos3ConfigFromRaw(raw string) *nacos.NacosConfig {
	return &nacos.NacosConfig{
		Type:          setting.SourceFromNacos3,
		ServerAddress: gjson.Get(raw, "server_address").String(),
		NacosAuthConfig: &nacos.NacosAuthConfig{
			UserName: gjson.Get(raw, "auth_config.user_name").String(),
			Password: gjson.Get(raw, "auth_config.password").String(),
		},
	}
}

func getNacosEEMSEAuthConfigFromRaw(raw string) *nacos.NacosConfig {
	return &nacos.NacosConfig{
		Type:          setting.SourceFromNacosEEMSE,
		ServerAddress: gjson.Get(raw, "server_address").String(),
		NacosAuthConfig: &nacos.NacosEEMSEAuthConfig{
			InstanceId:      gjson.Get(raw, "auth_config.instance_id").String(),
			AccessKeyId:     gjson.Get(raw, "auth_config.access_key_id").String(),
			AccessKeySecret: gjson.Get(raw, "auth_config.access_key_secret").String(),
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
			User:  gjson.Get(rawJson, "user").String(),
		}
	case setting.SourceFromNacos:
		management.AuthConfig = &nacos.NacosAuthConfig{
			UserName: gjson.Get(rawJson, "user_name").String(),
			Password: gjson.Get(rawJson, "password").String(),
		}
	case setting.SourceFromNacos3:
		management.AuthConfig = &nacos.NacosAuthConfig{
			UserName: gjson.Get(rawJson, "user_name").String(),
			Password: gjson.Get(rawJson, "password").String(),
		}
	case setting.SourceFromNacosEEMSE:
		management.AuthConfig = &nacos.NacosEEMSEAuthConfig{
			InstanceId:      gjson.Get(rawJson, "instance_id").String(),
			AccessKeyId:     gjson.Get(rawJson, "access_key_id").String(),
			AccessKeySecret: gjson.Get(rawJson, "access_key_secret").String(),
		}
	default:
		return errors.New("marshal auth config: invalid type")
	}
	return nil
}

func validateConfigurationManagementType(management *commonmodels.ConfigurationManagement) error {
	if management.Type != setting.SourceFromApollo &&
		management.Type != setting.SourceFromNacos &&
		management.Type != setting.SourceFromNacos3 &&
		management.Type != setting.SourceFromNacosEEMSE {
		return errors.New("invalid type")
	}
	return nil
}

func ListApolloApps(id string, log *zap.SugaredLogger) ([]string, error) {
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), id)
	if err != nil {
		return nil, errors.Errorf("failed to get apollo info from mongo: %v", err)
	}
	cli := apollo.NewClient(info.ServerAddress, info.Token)
	apps, err := cli.ListApp()
	if err != nil {
		return nil, e.ErrGetApolloInfo.AddErr(err)
	}
	var idList []string
	for _, app := range apps {
		idList = append(idList, app.AppID)
	}
	return idList, nil
}

func ListApolloEnvAndClusters(id string, appID string, log *zap.SugaredLogger) ([]*apollo.EnvAndCluster, error) {
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), id)
	if err != nil {
		return nil, errors.Errorf("failed to get apollo info from mongo: %v", err)
	}
	cli := apollo.NewClient(info.ServerAddress, info.Token)
	envs, err := cli.ListAppEnvsAndClusters(appID)
	if err != nil {
		return nil, e.ErrGetApolloInfo.AddErr(err)
	}
	return envs, nil
}

func ListApolloConfigByType(id, appID, format string, log *zap.SugaredLogger) ([]*apollo.BriefNamespace, error) {
	resp := make([]*apollo.BriefNamespace, 0)
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), id)
	if err != nil {
		log.Errorf("failed to get apollo info in database, error: %s", err)
		return nil, errors.Errorf("failed to get apollo info from mongo: %v", err)
	}
	cli := apollo.NewClient(info.ServerAddress, info.Token)
	envs, err := cli.ListAppEnvsAndClusters(appID)
	if err != nil {
		log.Errorf("failed to list env and clusters for app id: %s, error is: %s", id, err)
		return nil, e.ErrGetApolloInfo.AddErr(err)
	}
	for _, env := range envs {
		for _, cluster := range env.Clusters {
			namesapceList, err := cli.ListAppNamespace(appID, env.Env, cluster)
			if err != nil {
				log.Errorf("failed to get namespace info from apollo for app: %s, env: %s, cluster: %s, error: %s", appID, env.Env, cluster, err)
				return nil, e.ErrGetApolloInfo.AddErr(err)
			}
			for _, namespace := range namesapceList {
				if format != "" && namespace.Format != format {
					continue
				}
				resp = append(resp, &apollo.BriefNamespace{
					AppID:         appID,
					Env:           env.Env,
					ClusterName:   cluster,
					NamespaceName: namespace.NamespaceName,
					Format:        namespace.Format,
				})
			}
		}
	}

	return resp, nil
}

func ListApolloNamespaces(id string, appID string, env string, cluster string, log *zap.SugaredLogger) ([]string, error) {
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), id)
	if err != nil {
		return nil, errors.Errorf("failed to get apollo info from mongo: %v", err)
	}
	cli := apollo.NewClient(info.ServerAddress, info.Token)
	namespaces, err := cli.ListAppNamespace(appID, env, cluster)
	if err != nil {
		return nil, e.ErrGetApolloInfo.AddErr(err)
	}
	var namespaceNameList []string
	for _, v := range namespaces {
		namespaceNameList = append(namespaceNameList, v.NamespaceName)
	}
	return namespaceNameList, nil
}

func ListApolloConfig(id, appID, env, cluster, namespace string, log *zap.SugaredLogger) (*ApolloConfig, error) {
	info, err := mongodb.NewConfigurationManagementColl().GetApolloByID(context.Background(), id)
	if err != nil {
		log.Errorf("failed to get apollo info in database, error: %s", err)
		return nil, errors.Errorf("failed to get apollo info from mongo: %v", err)
	}
	cli := apollo.NewClient(info.ServerAddress, info.Token)
	namespaceInfo, err := cli.GetNamespace(appID, env, cluster, namespace)
	if err != nil {
		log.Errorf("failed to get namespace from apollo, error: %s", err)
		return nil, fmt.Errorf("failed to get namespace info from apollo, error: %s", err)
	}
	resp := make([]*commonmodels.ApolloKV, 0)
	for _, item := range namespaceInfo.Items {
		item.Key = apollo.NormalizeItemKey(namespaceInfo.Format, item.Key)
		resp = append(resp, &commonmodels.ApolloKV{
			Key: item.Key,
			Val: item.Value,
		})
	}
	return &ApolloConfig{
		ConfigType: namespaceInfo.Format,
		Config:     resp,
	}, nil
}

func ListNacosConfigByType(id, format string, log *zap.SugaredLogger) ([]*BriefNacosConfig, error) {
	resp := make([]*BriefNacosConfig, 0)
	cli, err := service.GetNacosClient(id)
	if err != nil {
		log.Errorf("failed to get nacos client, error is: %s", err)
		return nil, e.ErrGetNacosInfo.AddErr(err)
	}
	namespaces, err := cli.ListNamespaces()
	if err != nil {
		log.Errorf("failed to list nacos namespaces for id: %s, error is: %s", id, err)
		return nil, e.ErrGetNacosInfo.AddErr(err)
	}
	for _, namespace := range namespaces {
		configs, err := cli.ListConfigs(namespace.NamespaceID, "")
		if err != nil {
			log.Errorf("failed to list nacos config for namespace: %s, error is: %s", namespace.NamespacedName, err)
			return nil, e.ErrGetNacosInfo.AddErr(err)
		}
		for _, config := range configs {
			if format != "" && format != config.Format {
				continue
			}
			resp = append(resp, &BriefNacosConfig{
				DataID:        config.DataID,
				Format:        config.Format,
				Group:         config.Group,
				NamespaceID:   namespace.NamespaceID,
				NamespaceName: namespace.NamespacedName,
			})
		}
	}

	return resp, nil
}

func GetNacosConfig(id, namespace, groupName, dataName string, log *zap.SugaredLogger) (*types.NacosConfig, error) {
	cli, err := service.GetNacosClient(id)
	if err != nil {
		log.Errorf("failed to get nacos client, error is: %s", err)
		return nil, e.ErrGetNacosInfo.AddErr(err)
	}
	resp, err := cli.GetConfig(dataName, groupName, namespace)
	if err != nil {
		log.Errorf("failed to get config info for nacos data: %s under namespace: %s, error: %s", dataName, namespace, err)
		return nil, e.ErrGetNacosInfo.AddErr(err)
	}
	return resp, nil
}
