/*
Copyright 2021 The KodeRover Authors.

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

package service

import (
	"net/http"
	"net/url"
	"strings"

	"go.uber.org/zap"

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/shared/client/policy"
	"github.com/koderover/zadig/pkg/tool/log"
)

type CreateProjectArgs struct {
	Public      bool     `json:"public"`
	ProductName string   `json:"product_name"`
	Admins      []string `json:"admins"`
}

type allowedProjectsData struct {
	Result []string `json:"result"`
}

func CreateProject(header http.Header, body []byte, qs url.Values, args *CreateProjectArgs, logger *zap.SugaredLogger) ([]byte, error) {
	var rbs []*policy.RoleBinding
	for _, uid := range args.Admins {
		rbs = append(rbs, &policy.RoleBinding{
			Name:   configbase.RoleBindingNameFromUIDAndRole(uid, setting.ProjectAdmin, ""),
			UID:    uid,
			Role:   string(setting.ProjectAdmin),
			Preset: true,
		})
	}

	if args.Public {
		rbs = append(rbs, &policy.RoleBinding{
			Name:   configbase.RoleBindingNameFromUIDAndRole("*", setting.ReadOnly, ""),
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Preset: true,
		})
	}

	policyClient := policy.NewDefault()
	for _, rb := range rbs {
		if err := policyClient.CreateOrUpdateRoleBinding(args.ProductName, rb); err != nil {
			logger.Errorf("Failed to create rolebinding %s, err: %s", rb.Name, err)
			return nil, err
		}
	}

	res, err := aslan.New().CreateProject(header, qs, body)
	if err != nil {
		logger.Errorf("Failed to create project %s, err: %s", args.ProductName, err)
		for _, rb := range rbs {
			if err1 := policyClient.DeleteRoleBinding(rb.Name, args.ProductName); err1 != nil {
				logger.Errorf("Failed to create rolebinding %s, err: %s", rb.Name, err1)
			}
		}

		return nil, err
	}

	return res, nil
}

func UpdateProject(header http.Header, qs url.Values, body []byte, projectName string, public bool, logger *zap.SugaredLogger) ([]byte, error) {
	// role binding
	roleBindingName := configbase.RoleBindingNameFromUIDAndRole("*", setting.ReadOnly, "")
	if !public {
		if err := policy.NewDefault().DeleteRoleBinding(roleBindingName, projectName); err != nil {
			logger.Errorf("Failed to delete role binding, err: %s", err)
			return nil, err
		}
	} else {
		if err := policy.NewDefault().CreateOrUpdateRoleBinding(projectName, &policy.RoleBinding{
			Name:   roleBindingName,
			UID:    "*",
			Role:   string(setting.ReadOnly),
			Preset: true,
		}); err != nil {
			logger.Errorf("Failed to create role binding, err: %s", err)
			return nil, err
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

	if !(len(names) == 1 && names[0] == "*") {
		for _, name := range names {
			qs.Add("names", name)
		}
	}

	aslanClient := aslan.New()

	return aslanClient.ListProjects(header, qs)
}

func DeleteProject(header http.Header, qs url.Values, productName string, logger *zap.SugaredLogger) ([]byte, error) {
	// delete roles and rolebindings
	if err := policy.NewDefault().DeleteRoleBindings([]string{"*"}, productName); err != nil {
		log.Errorf("delete rolebindings err:%s", err)
	}
	if err := policy.NewDefault().DeleteRoles([]string{"*"}, productName); err != nil {
		log.Errorf("delete roles err:%s", err)
	}
	return aslan.New().DeleteProject(header, qs, productName)
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
