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
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/shared/client/plutusvendor"
	"github.com/koderover/zadig/pkg/shared/client/user"
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type SystemInitializationStatus struct {
	Initialized   bool   `json:"initialized"`
	IsEnterprise  bool   `json:"is_enterprise"`
	LicenseStatus string `json:"license_status"`
	SystemID      string `json:"system_id"`
}

func GetSystemInitializationStatus(logger *zap.SugaredLogger) (*SystemInitializationStatus, error) {
	// first check if the system is enterprise version
	isEnterprise := config.Enterprise()

	// then check if the user has been initialized
	userCountInfo, err := user.New().CountUsers()
	if err != nil {
		logger.Errorf("failed to get user count, error: %s", err)
		return nil, fmt.Errorf("failed to check if the user is initialized, error: %s", err)
	}

	resp := &SystemInitializationStatus{
		IsEnterprise: isEnterprise,
	}

	if userCountInfo.TotalUser > 0 {
		resp.Initialized = true
	} else {
		resp.Initialized = false
	}

	// if it is an enterprise system, check about the license information
	if isEnterprise {
		licenseInfo, err := plutusvendor.New().CheckZadigXLicenseStatus()
		if err != nil {
			logger.Errorf("failed to get enterprise license info, error: %s", err)
			return nil, fmt.Errorf("failed to check enterprise license info, error: %s", err)
		}
		resp.LicenseStatus = licenseInfo.Status
		resp.SystemID = licenseInfo.SystemID
	}

	return resp, nil
}

func InitializeUser(username, password, company, email string, phone int64, reason, address string, logger *zap.SugaredLogger) error {
	userCountInfo, err := user.New().CountUsers()
	if err != nil {
		logger.Errorf("failed to get user count, error: %s", err)
		return fmt.Errorf("failed to check if the user is initialized, error: %s", err)
	}

	if userCountInfo.TotalUser > 0 {
		return nil
	}

	userInfo, err := user.New().CreateUser(&user.CreateUserArgs{
		Name:     username,
		Password: password,
		Email:    email,
		Phone:    strconv.FormatInt(phone, 10),
		Account:  username,
	})

	if err != nil {
		logger.Errorf("failed to create user, error: %s", err)
		return fmt.Errorf("user initialization error: failed to create user, err: %s", err)
	}

	if !config.Enterprise() {
		initializeInfo := &InitializeInfo{
			CreatedAt: time.Now().Unix(),
			Username:  username,
			Phone:     phone,
			Email:     email,
			Company:   company,
			Reason:    reason,
			Address:   address,
			Domain:    config.SystemAddress(),
		}

		err = reportRegister(initializeInfo)
		if err != nil {
			// don't stop the whole initialization process if the upload fails
			logger.Errorf("failed to upload initialization info, error: %s", err)
		}
	}

	// this role must exist since when this api is working, user service has already done the initialization.
	return user.New().CreateUserRoleBinding(userInfo.Uid, "*", "admin")
}

type InitializeInfo struct {
	CreatedAt int64  `json:"created_at"`
	Username  string `json:"username"`
	Phone     int64  `json:"phone,omitempty"`
	Email     string `json:"email"`
	Company   string `json:"company"`
	Reason    string `json:"reason,omitempty"`
	Address   string `json:"address,omitempty"`
	Domain    string `json:"domain"`
}

type Operation struct {
	Data string `json:"data"`
}

func reportRegister(info *InitializeInfo) error {
	_, err := httpclient.Post("https://api.koderover.com/api/operation/admin/user", httpclient.SetBody(info))
	return err
}
