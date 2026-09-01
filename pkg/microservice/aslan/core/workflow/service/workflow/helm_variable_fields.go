package workflow

import (
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	servicerepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/repository"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/helmclient"
	"github.com/koderover/zadig/v2/pkg/util/converter"
)

type HelmVariableFieldsRequest struct {
	OverrideYaml   string                  `json:"overrideYaml" yaml:"overrideYaml"`
	OverrideValues []*commonservice.KVPair `json:"overrideValues,omitempty" yaml:"overrideValues,omitempty"`
}

type HelmVariableFieldsResponse struct {
	LatestFlatMap map[string]interface{} `json:"latest_flat_map"`
}

// GetHelmVariableFields resolves template values and optional environment and
// request overrides. The template is the source of truth when the service has
// not been created in the environment yet.
func GetHelmVariableFields(projectName, serviceName, envName string, production bool, args *HelmVariableFieldsRequest, logger *zap.SugaredLogger) (*HelmVariableFieldsResponse, error) {
	if args == nil {
		args = &HelmVariableFieldsRequest{}
	}

	templateService, err := servicerepo.QueryTemplateService(&commonrepo.ServiceFindOption{
		ServiceName: serviceName,
		ProductName: projectName,
		Type:        setting.HelmDeployType,
	}, production)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find helm service %s in project %s", serviceName, projectName)
	}
	if templateService.HelmChart == nil {
		return nil, fmt.Errorf("service %s in project %s is not a helm service", serviceName, projectName)
	}

	valuesYAML := templateService.HelmChart.ValuesYaml
	if envName != "" {
		productionValue := production
		env, findErr := commonrepo.NewProductColl().Find(&commonrepo.ProductFindOptions{
			Name:       projectName,
			EnvName:    envName,
			Production: &productionValue,
		})
		if findErr != nil {
			return nil, errors.Wrapf(findErr, "failed to find environment %s", envName)
		}

		var envYAML, envOverrideValues string
		if envService := env.GetServiceMap()[serviceName]; envService != nil {
			render := envService.GetServiceRender()
			envYAML = render.GetSafeVariable()
			envOverrideValues = render.OverrideValues
		}
		valuesYAML, err = helmclient.MergeOverrideValues(valuesYAML, env.DefaultValues, envYAML, envOverrideValues, nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to merge environment helm values")
		}
	}

	overrideValues := (&commonservice.HelmSvcRenderArg{OverrideValues: args.OverrideValues}).ToOverrideValueString()
	valuesYAML, err = helmclient.MergeOverrideValues(valuesYAML, "", args.OverrideYaml, overrideValues, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to merge request helm values")
	}

	response, err := helmVariableFlatMapFromYAML(valuesYAML)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse helm values")
	}
	if logger != nil {
		logger.Debugw("resolved helm variable fields", "projectName", projectName, "serviceName", serviceName, "envName", envName)
	}
	return response, nil
}

// helmVariableFlatMapFromYAML is kept independent from MongoDB so the value
// flattening can be tested without external dependencies.
func helmVariableFlatMapFromYAML(valuesYAML string) (*HelmVariableFieldsResponse, error) {
	flatMap, err := converter.YamlToFlatMap([]byte(valuesYAML))
	if err != nil {
		return nil, err
	}
	return &HelmVariableFieldsResponse{LatestFlatMap: flatMap}, nil
}
