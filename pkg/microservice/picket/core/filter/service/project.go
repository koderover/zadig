package service

import (
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
	"github.com/koderover/zadig/pkg/setting"
)

type allowedProjectsData struct {
	Result []string `json:"result"`
}

func ListProjects(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	names, err := getAllowedProjects(header, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range names {
		qs.Add("names", name)
	}

	aslanClient := aslan.New()

	return aslanClient.ListProjects(header, qs)
}

func getAllowedProjects(headers http.Header, logger *zap.SugaredLogger) ([]string, error) {
	res := &allowedProjectsData{}

	opaClient := opa.NewDefault()
	err := opaClient.Evaluate("rbac.user_projects", res, func() (*opa.Input, error) { return generateOPAInput(headers), nil })
	if err != nil {
		logger.Errorf("opa evaluation failed, err: %s", err)
		return nil, err
	}

	return res.Result, nil

}

func generateOPAInput(header http.Header) *opa.Input {
	authorization := header.Get(strings.ToLower(setting.AuthorizationHeader))
	headers := map[string]string{}
	headers[strings.ToLower(setting.AuthorizationHeader)] = authorization

	return &opa.Input{
		Attributes: &opa.Attributes{
			Request: &opa.Request{HTTP: &opa.HTTPSpec{
				Headers: headers,
			}},
		},
	}
}
