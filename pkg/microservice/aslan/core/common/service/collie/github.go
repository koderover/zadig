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

package collie

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type webHook struct {
	Payload     string `json:"payload"`
	EventType   string `json:"event_type,omitempty"`
	MessageType string `json:"message_type,omitempty"`
}

func CallGithubWebHook(forwardedProto, forwardedHost string, payload []byte, messageType string, log *zap.SugaredLogger) error {
	collieAPIAddress := config.CollieAPIAddress()
	if collieAPIAddress == "" {
		return nil
	}

	wh := &webHook{Payload: string(payload), MessageType: messageType}
	_, err := httpclient.Post(
		fmt.Sprintf("%s/api/collie/api/hook/github", collieAPIAddress),
		httpclient.SetBody(wh),
		httpclient.SetHeader("X-Forwarded-Proto", forwardedProto),
		httpclient.SetHeader("X-Forwarded-Host", forwardedHost),
	)
	if err != nil {
		log.Errorf("call collie github webhook err:%+v", err)
		return err
	}

	return nil
}
