/*
Copyright 2022 The KodeRover Authors.

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

package types

type Probe struct {
	ProbeScheme string         `bson:"probe_type"                 json:"probe_type"`
	HttpProbe   *HTTPGetAction `bson:"http_probe"                 json:"http_probe"`
}

type HTTPHeader struct {
	Name  string `bson:"name"                 json:"name"`
	Value string `bson:"value"                json:"value"`
}

type HTTPGetAction struct {
	Path                string        `bson:"path"                        json:"path"`
	Port                int           `bson:"port"                        json:"port"`
	Host                string        `bson:"-"                           json:"host,omitempty"`
	HTTPHeaders         []*HTTPHeader `bson:"http_headers"                json:"http_headers"`
	TimeOutSecond       int           `bson:"timeout_second"              json:"timeout_second"`
	ResponseSuccessFlag string        `bson:"response_success_flag"       json:"response_success_flag"`
}
