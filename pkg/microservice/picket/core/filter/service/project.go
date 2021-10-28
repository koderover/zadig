package service

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
)

type allowedProjectsData struct {
	Result []string `json:"result"`
}

func CreateProject(header http.Header, body []byte, projectName string, public bool, logger *zap.SugaredLogger) ([]byte, error) {
	// role binding
	roleBindingName := fmt.Sprintf(setting.RoleBindFmt, "*", setting.ReadOnly)
	if public {

		err := policy.NewDefault().CreateRoleBinding(projectName, &policy.RoleBinding{
			Name:   roleBindingName,
			User:   "*",
			Role:   setting.ReadOnly,
			Public: true,
		})
		logger.Errorf("create rolebinding: %s err: %s", roleBindingName, err)
	}

	res, err := aslan.New().CreateProject(header, body, projectName)
	if err != nil {
		policy.NewDefault().DeleteRoleBinding(roleBindingName, projectName)
		logger.Errorf("delete rolebinding: %s err: %s", roleBindingName, err)
	}
	return res, err
}

func ListProjects(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	names, err := getVisibleProjects(header, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	if len(names) == 0 {
		return []byte("[]"), nil
	}

	for _, name := range names {
		qs.Add("names", name)
	}

	aslanClient := aslan.New()

	return aslanClient.ListProjects(header, qs)
}

func getVisibleProjects(headers http.Header, logger *zap.SugaredLogger) ([]string, error) {
	res := &allowedProjectsData{}
	opaClient := opa.NewDefault()
	err := opaClient.Evaluate("rbac.user_visible_projects", res, func() (*opa.Input, error) { return generateOPAInput(headers, "", ""), nil })
	if err != nil {
		logger.Errorf("opa evaluation failed, err: %s", err)
		return nil, err
	}

	return res.Result, nil
}

func generateOPAInput(header http.Header, method string, endpoint string) *opa.Input {
	authorization := header.Get(strings.ToLower(setting.AuthorizationHeader))
	headers := map[string]string{}
	parsedPath := strings.Split(strings.Trim(endpoint, "/"), "/")
	headers[strings.ToLower(setting.AuthorizationHeader)] = authorization

	return &opa.Input{
		Attributes: &opa.Attributes{
			Request: &opa.Request{HTTP: &opa.HTTPSpec{
				Headers: headers,
				Method:  method,
			}},
		},
		ParsedPath: parsedPath,
	}
}
