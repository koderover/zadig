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

package handler

import (
	"net/http"

	"github.com/koderover/zadig/pkg/microservice/hubserver/core/service"
	"github.com/koderover/zadig/pkg/tool/remotedialer"
)

func Disconnect(server *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	service.Disconnect(server, w, r)
}

func Restore(w http.ResponseWriter, r *http.Request) {
	service.Restore(w, r)
}

func Forward(server *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	service.Forward(server, w, r)
}

func HasSession(handler *remotedialer.Server, w http.ResponseWriter, r *http.Request) {
	service.HasSession(handler, w, r)
}
