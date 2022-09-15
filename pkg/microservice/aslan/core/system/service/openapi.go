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
	"time"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
)

func OpenAPICreateRegistry(username string, req *OpenAPICreateRegistryReq, logger *zap.SugaredLogger) error {
	reg := &commonmodels.RegistryNamespace{
		RegAddr:     req.Address,
		RegProvider: string(req.Provider),
		IsDefault:   req.IsDefault,
		Namespace:   req.Namespace,
		AccessKey:   req.AccessKey,
		SecretKey:   req.SecretKey,
		Region:      req.Region,
		UpdateTime:  time.Now().Unix(),
		UpdateBy:    username,
		AdvancedSetting: &commonmodels.RegistryAdvancedSetting{
			Modified:   true,
			TLSEnabled: req.EnableTLS,
			TLSCert:    req.TLSCert,
		},
	}

	return CreateRegistryNamespace(username, reg, logger)
}
