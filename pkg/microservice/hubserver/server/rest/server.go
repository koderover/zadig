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

package rest

import (
	"net/http"

	"github.com/gorilla/mux"

	h "github.com/koderover/zadig/pkg/microservice/hubserver/core/handler"
	"github.com/koderover/zadig/pkg/tool/remotedialer"
)

type engine struct {
	*mux.Router
}

func NewEngine(handler *remotedialer.Server) *engine {
	s := &engine{}
	s.Router = mux.NewRouter()
	s.Router.UseEncodedPath()

	s.injectRouters(handler)

	return s
}

func (s *engine) injectRouters(handler *remotedialer.Server) {
	r := s.Router

	r.Handle("/connect", handler)

	r.HandleFunc("/disconnect/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.Disconnect(handler, rw, req)
	})

	r.HandleFunc("/restore/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.Restore(rw, req)
	})

	r.HandleFunc("/hasSession/{id}", func(rw http.ResponseWriter, req *http.Request) {
		h.HasSession(handler, rw, req)
	})

	r.HandleFunc("/kube/{id}{path:.*}", func(rw http.ResponseWriter, req *http.Request) {
		h.Forward(handler, rw, req)
	})

	s.Router = r
}
