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

	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
	"go.uber.org/zap"
)

type Input struct {
	ParsedQuery ParseQuery `json:"parsed_query"`
	ParsedPath  []string   `json:"parsed_path"`
	Attributes  Attributes `json:"attributes"`
}
type ParseQuery struct {
	ProjectName []string `json:"projectName"`
}

type Attributes struct {
	Request Request `json:"request"`
}

type Request struct {
	Http HTTP `json:"http"`
}

type HTTP struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

type OpaRes struct {
	Result bool `json:"result"`
}

func Evaluate(logger *zap.SugaredLogger, header http.Header, projectName string, grantsReq []GrantReq) (interface{}, error) {
	// 拼接参数，请求opa
	authorization := header.Get("authorization")
	opaHeaders := map[string]string{}
	opaHeaders["authorization"] = authorization

	var grantsRes []GrantRes
	// 对于每一个action+endpoint 都去请求opa
	for _, v := range grantsReq {
		parsedPath := strings.Split(v.EndPoint, "/")
		input := Input{
			ParsedQuery: ParseQuery{
				ProjectName: []string{projectName},
			},
			ParsedPath: parsedPath,
			Attributes: Attributes{
				Request: Request{Http: HTTP{
					Method:  v.Method,
					Headers: opaHeaders,
				}},
			},
		}
		res, err := opa.NewDefault().Evaluate("rbac.allow", input)
		if err != nil {
			logger.Errorf("opa evaluate err %s", err)
			continue
		}
		var opaR OpaRes
		if err := json.Unmarshal(res, &opaR); err != nil {
			logger.Errorf("opa res Unmarshal err %s", err)
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
