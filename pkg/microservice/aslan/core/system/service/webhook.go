/*
Copyright 2023 The KodeRover Authors.

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
	"context"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	gitservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/git"
	"go.uber.org/zap"
)

type GetWebhookConfigReponse struct {
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

func GetWebhookConfig(ctx context.Context, log *zap.SugaredLogger) (*GetWebhookConfigReponse, error) {
	resp := &GetWebhookConfigReponse{
		URL:    config.WebHookURL(),
		Secret: gitservice.GetHookSecret(),
	}
	return resp, nil
}
