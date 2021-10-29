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

func CreateProject(header http.Header, body []byte, qs url.Values, projectName string, public bool, logger *zap.SugaredLogger) ([]byte, error) {
	// role binding
	roleBindingName := fmt.Sprintf(setting.RoleBindingNameFmt, "*", setting.ReadOnly, projectName)
	if public {
		if err := policy.NewDefault().CreateRoleBinding(projectName, &policy.RoleBinding{
			Name:   roleBindingName,
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Public: true,
		}); err != nil {
			logger.Errorf("Failed to create rolebinding %s, err: %s", roleBindingName, err)
			return nil, err
		}
	}

	res, err := aslan.New().CreateProject(header, qs, body)
	if err != nil {
		logger.Errorf("Failed to create project %s, err: %s", projectName, err)
		if err1 := policy.NewDefault().DeleteRoleBinding(roleBindingName, projectName); err1 != nil {
			logger.Warnf("Failed to delete role binding, err: %s", err1)
		}

		return nil, err
	}

	return res, nil
}

func UpdateProject(header http.Header, qs url.Values, body []byte, projectName string, public bool, logger *zap.SugaredLogger) ([]byte, error) {
	// role binding
	roleBindingName := fmt.Sprintf(setting.RoleBindingNameFmt, "*", setting.ReadOnly, projectName)
	if !public {
		if err := policy.NewDefault().DeleteRoleBinding(roleBindingName, projectName); err != nil {
			logger.Warnf("Failed to delete role binding, err: %s", err)
		}
	} else {
		if err := policy.NewDefault().CreateRoleBinding(projectName, &policy.RoleBinding{
			Name:   roleBindingName,
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Public: true,
		}); err != nil {
			logger.Warnf("Failed to create role binding, err: %s", err)
		}
	}

	res, err := aslan.New().UpdateProject(header, qs, body)
	if err != nil {
		logger.Errorf("Failed to create project %s, err: %s", projectName, err)
		return nil, err
	}

	return res, nil
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
