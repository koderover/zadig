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

package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func Overview(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Overview(header, qs)
}

func Test(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Test(header, qs)
}

func Deploy(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Deploy(header, qs)
}

func Build(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Build(header, qs)
}
