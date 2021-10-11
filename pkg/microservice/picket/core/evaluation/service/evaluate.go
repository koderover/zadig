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
	"fmt"
	"net/http"

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
	Method  string      `json:"method"`
	Headers http.Header `json:"headers"`
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func Evaluate(logger *zap.SugaredLogger, header http.Header, projectName string) (interface{}, error) {
	// 拼接参数，请求opa
	opaHeaders := http.Header{}
	copyHeader(opaHeaders, header)
	input := Input{
		ParsedQuery: ParseQuery{
			ProjectName: []string{projectName},
		},
		ParsedPath: nil,
		Attributes: Attributes{
			Request: Request{Http: HTTP{
				Method:  "",
				Headers: opaHeaders,
			}},
		},
	}
	fmt.Printf("%+v", input)
	return opa.NewDefault().Evaluate("rbac.projcet_name", input)
}
