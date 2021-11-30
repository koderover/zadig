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

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/microservice/picket/client/user"
)

type DeleteUserResp struct {
	Message string `json:"message"`
}

func DeleteUser(userID string, header http.Header, qs url.Values, _ *zap.SugaredLogger) ([]byte, error) {
	_, err := user.New().DeleteUser(userID, header, qs)
	if err != nil {
		return []byte{}, err
	}
	return policy.New().DeleteRoleBindings(userID, header, qs)
}
