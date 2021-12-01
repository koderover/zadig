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

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func CreateWorkflowTask(header http.Header, qs url.Values, body []byte, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().CreateWorkflowTask(header, qs, body)
}

func CancelWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().CancelWorkflowTask(header, qs, id, name)
}

func RestartWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().RestartWorkflowTask(header, qs, id, name)
}

func ListWorkflowTask(header http.Header, qs url.Values, commitId string, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().ListWorkflowTask(header, qs, commitId)
}

func ListDelivery(header http.Header, qs url.Values, productName string, workflowName string, taskId string, perPage string, page string, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().ListDelivery(header, qs, productName, workflowName, taskId, perPage, page)
}
