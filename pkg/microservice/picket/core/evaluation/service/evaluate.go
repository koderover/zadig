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
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
	"github.com/koderover/zadig/pkg/setting"
)

type evaluateResult struct {
	Result bool `json:"result"`
}

func Evaluate(header http.Header, projectName string, grantsReq []GrantReq, logger *zap.SugaredLogger) ([]GrantRes, error) {

	var grantsRes []GrantRes
	opaClient := opa.NewDefault()
	// 对于每一个action+endpoint 都去请求opa
	for _, v := range grantsReq {
		parsedPath := strings.Split(strings.Trim(v.EndPoint, "/"), "/")
		res := &evaluateResult{}
		err := opaClient.Evaluate(
			"rbac.allow", res,
			func() (*opa.Input, error) { return generateOPAInput(header, projectName, v.Method, parsedPath), nil })
		if err != nil {
			logger.Errorf("opa evaluate endpoint: %v method: %v err: %s", v.EndPoint, v.Method, err)
			grantsRes = append(grantsRes, GrantRes{v, false})
			continue
		}
		grantsRes = append(grantsRes, GrantRes{v, res.Result})
	}
	return grantsRes, nil
}

type GrantReq struct {
	EndPoint string `json:"endpoint"`
	Method   string `json:"method"`
}

type GrantRes struct {
	GrantReq
	Allow bool `json:"allow"`
}

func generateOPAInput(header http.Header, projectName, method string, parsedPath []string) *opa.Input {
	authorization := header.Get(strings.ToLower(setting.AuthorizationHeader))
	headers := map[string]string{}
	headers[strings.ToLower(setting.AuthorizationHeader)] = authorization

	return &opa.Input{
		ParsedQuery: &opa.ParseQuery{
			ProjectName: []string{projectName},
		},
		ParsedPath: parsedPath,
		Attributes: &opa.Attributes{
			Request: &opa.Request{HTTP: &opa.HTTPSpec{
				Method:  method,
				Headers: headers,
			}},
		},
	}
}
