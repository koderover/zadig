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

package opa

type Input struct {
	ParsedQuery *ParseQuery `json:"parsed_query"`
	ParsedPath  []string    `json:"parsed_path"`
	Attributes  *Attributes `json:"attributes"`
}
type ParseQuery struct {
	ProjectName []string `json:"projectName"`
}

type Attributes struct {
	Request *Request `json:"request"`
}

type Request struct {
	HTTP *HTTPSpec `json:"http"`
}

type HTTPSpec struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
}
