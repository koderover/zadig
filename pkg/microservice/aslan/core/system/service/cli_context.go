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
	"os"
	"strings"

	"github.com/koderover/zadig/v2/pkg/shared/client/plutusenterprise"
	"github.com/koderover/zadig/v2/pkg/types"
	"go.uber.org/zap"
)

const (
	cliContextUnknownEdition          = "unknown"
	cliContextUnavailableLicenseState = "unavailable"
)

type CLIContextResponse struct {
	Principal     CLIPrincipal `json:"principal"`
	Edition       string       `json:"edition"`
	LicenseStatus string       `json:"license_status"`
	Features      []string     `json:"features"`
	ServerVersion string       `json:"server_version"`
	RequestID     string       `json:"request_id"`
}

// CLIPrincipal identifies the authenticated user represented by the request token.
type CLIPrincipal struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Account      string `json:"account"`
	IdentityType string `json:"identity_type"`
}

func GetCLIContext(principal types.UserBriefInfo, requestID string, log *zap.SugaredLogger) (*CLIContextResponse, error) {
	response := &CLIContextResponse{
		Principal: CLIPrincipal{
			UID:          principal.UID,
			Name:         principal.Name,
			Account:      principal.Account,
			IdentityType: principal.IdentityType,
		},
		Edition:       cliContextUnknownEdition,
		LicenseStatus: cliContextUnavailableLicenseState,
		Features:      []string{},
		ServerVersion: strings.TrimSpace(os.Getenv("VERSION")),
		RequestID:     requestID,
	}

	licenseStatus, err := plutusenterprise.New().CheckZadigXLicenseStatus()
	if err != nil {
		if log != nil {
			log.Warnw("failed to load CLI license metadata", "request_id", requestID, "error", err)
		}
		return response, nil
	}
	if licenseStatus == nil {
		if log != nil {
			log.Warnw("received empty CLI license metadata", "request_id", requestID)
		}
		return response, nil
	}

	if value := strings.TrimSpace(licenseStatus.Type); value != "" {
		response.Edition = value
	}
	if value := strings.TrimSpace(licenseStatus.Status); value != "" {
		response.LicenseStatus = value
	}
	if value := strings.TrimSpace(licenseStatus.CurrentVersion); value != "" {
		response.ServerVersion = value
	}
	response.Features = append([]string{}, licenseStatus.Features...)
	return response, nil
}
