/*
Copyright 2026 The KodeRover Authors.

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
	"errors"
	"fmt"

	"github.com/koderover/zadig/v2/pkg/shared/client/plutusenterprise"
	"github.com/koderover/zadig/v2/pkg/types"
)

type CLIContextResponse struct {
	Principal     CLIPrincipal `json:"principal"`
	Edition       string       `json:"edition"`
	LicenseStatus string       `json:"license_status"`
	Features      []string     `json:"features"`
	ServerVersion string       `json:"server_version"`
	RequestID     string       `json:"request_id"`
}

type CLIPrincipal struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Account      string `json:"account"`
	IdentityType string `json:"identity_type"`
}

type licenseStatusChecker interface {
	CheckZadigXLicenseStatus() (*plutusenterprise.ZadigXLicenseStatus, error)
}

func GetCLIContext(principal types.UserBriefInfo, requestID string) (*CLIContextResponse, error) {
	return getCLIContext(principal, requestID, plutusenterprise.New())
}

func getCLIContext(principal types.UserBriefInfo, requestID string, checker licenseStatusChecker) (*CLIContextResponse, error) {
	licenseStatus, err := checker.CheckZadigXLicenseStatus()
	if err != nil {
		return nil, fmt.Errorf("check zadig license status: %w", err)
	}
	if licenseStatus == nil {
		return nil, errors.New("check zadig license status: empty response")
	}

	return &CLIContextResponse{
		Principal: CLIPrincipal{
			UID:          principal.UID,
			Name:         principal.Name,
			Account:      principal.Account,
			IdentityType: principal.IdentityType,
		},
		Edition:       licenseStatus.Type,
		LicenseStatus: licenseStatus.Status,
		Features:      append([]string{}, licenseStatus.Features...),
		ServerVersion: licenseStatus.CurrentVersion,
		RequestID:     requestID,
	}, nil
}
