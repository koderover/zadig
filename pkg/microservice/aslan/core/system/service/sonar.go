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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func CreateSonarIntegration(args *SonarIntegration, log *zap.SugaredLogger) error {
	err := commonrepo.NewSonarIntegrationColl().Create(context.TODO(), &commonmodels.SonarIntegration{
		ServerAddress: args.ServerAddress,
		Token:         args.Token,
	})
	if err != nil {
		log.Errorf("Create external system error: %s", err)
		return e.ErrCreateExternalLink.AddErr(err)
	}
	return nil
}

func UpdateSonarIntegration(id string, integration *SonarIntegration, log *zap.SugaredLogger) error {
	err := commonrepo.NewSonarIntegrationColl().Update(
		context.TODO(),
		id,
		&commonmodels.SonarIntegration{
			ServerAddress: integration.ServerAddress,
			Token:         integration.Token,
		},
	)
	if err != nil {
		log.Errorf("update external system error: %s", err)
	}
	return err
}

func ListSonarIntegration(log *zap.SugaredLogger) ([]*SonarIntegration, int64, error) {
	// for now paging is not supported
	sonarList, length, err := commonrepo.NewSonarIntegrationColl().List(context.TODO(), 0, 0)
	if err != nil {
		log.Errorf("Failed to list sonar integration from db, the error is: %s", err)
		return nil, 0, err
	}
	resp := make([]*SonarIntegration, 0)
	for _, sonar := range sonarList {
		resp = append(resp, &SonarIntegration{
			ID:            sonar.ID.Hex(),
			ServerAddress: sonar.ServerAddress,
			Token:         sonar.Token,
		})
	}
	return resp, length, nil
}

func GetSonarIntegration(id string, log *zap.SugaredLogger) (*SonarIntegration, error) {
	resp := new(SonarIntegration)
	sonarIntegration, err := commonrepo.NewSonarIntegrationColl().GetByID(context.TODO(), id)
	if err != nil {
		log.Errorf("Failed to get sonar integration detail from id %s, the error is: %s", id, err)
		return nil, err
	}
	resp.ID = sonarIntegration.ID.Hex()
	resp.ServerAddress = sonarIntegration.ServerAddress
	resp.Token = sonarIntegration.Token
	return resp, nil
}

func DeleteSonarIntegration(id string, log *zap.SugaredLogger) error {
	err := commonrepo.NewSonarIntegrationColl().DeleteByID(context.TODO(), id)
	if err != nil {
		log.Errorf("Failed to delete sonar integration of id: %s, the error is: %s", id, err)
	}
	return err
}

type sonarValidationResponse struct {
	Valid bool `json:"valid"`
}

func ValidateSonarIntegration(arg *SonarIntegration, log *zap.SugaredLogger) error {
	validateAPI := fmt.Sprintf("%s/%s", arg.ServerAddress, "api/authentication/validate")

	req, err := http.NewRequest("GET", validateAPI, nil)
	if err != nil {
		log.Errorf("failed to create http request to API: %s with error: %s", validateAPI, err)
		return err
	}

	req.SetBasicAuth(arg.Token, "")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("failed to request the sonar validate API, the error is: %s", err)
		return err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("failed to read response body, the error is: %s", err)
		return err
	}
	if resp.StatusCode == http.StatusOK {
		res := &sonarValidationResponse{}
		err = json.Unmarshal(bodyBytes, res)
		if err != nil {
			log.Errorf("failed to ready response body, the error is: %s", err)
			return err
		}
		if res.Valid {
			return nil
		}
		return errors.New("the token validation failed.")
	}

	return fmt.Errorf("the server responded with code: %d and message: %s", resp.StatusCode, string(bodyBytes))
}
