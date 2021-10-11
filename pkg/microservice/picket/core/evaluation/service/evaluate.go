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
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
)

type input struct {
	ParsedQuery parseQuery `json:"parsed_query"`
	ParsedPath  []string   `json:"parsed_path"`
	Attributes  attributes `json:"attributes"`
}
type parseQuery struct {
	ProjectName []string `json:"projectName"`
}

type attributes struct {
	Request request `json:"request"`
}

type request struct {
	Http HTTP `json:"http"`
}

type HTTP struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

type opaRes struct {
	Result bool `json:"result"`
}

func Evaluate(header http.Header, projectName string, grantsReq []GrantReq, logger *zap.SugaredLogger) ([]GrantRes, error) {
	// 拼接参数，请求opa
	authorization := header.Get("authorization")
	opaHeaders := map[string]string{}
	opaHeaders["authorization"] = authorization

	var grantsRes []GrantRes
	opaClient := opa.NewDefault()
	// 对于每一个action+endpoint 都去请求opa
	for _, v := range grantsReq {
		parsedPath := strings.Split(strings.Trim(v.EndPoint, "/"), "/")
		input := generateOPAInput(projectName, parsedPath, v.Method, opaHeaders)
		res, err := opaClient.Evaluate("rbac.allow", input)
		if err != nil {
			logger.Errorf("opa evaluate endpoint: %v method: %v err: %s", v.EndPoint, v.Method, err)
			grantsRes = append(grantsRes, GrantRes{v, false})
			continue
		}
		var opaR opaRes
		if err := json.Unmarshal(res, &opaR); err != nil {
			logger.Errorf("opa res Unmarshal err %s", err)
			grantsRes = append(grantsRes, GrantRes{v, false})
			continue
		}
		grantsRes = append(grantsRes, GrantRes{v, opaR.Result})
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

func generateOPAInput(projectName string, parsedPath []string, method string, head map[string]string) input {
	return input{
		ParsedQuery: parseQuery{
			ProjectName: []string{projectName},
		},
		ParsedPath: parsedPath,
		Attributes: attributes{
			Request: request{Http: HTTP{
				Method:  method,
				Headers: head,
			}},
		},
	}
}
