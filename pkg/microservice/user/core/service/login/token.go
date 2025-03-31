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

package login

import (
	"fmt"

	"github.com/golang-jwt/jwt"

	"github.com/koderover/zadig/v2/pkg/config"
	userconfig "github.com/koderover/zadig/v2/pkg/microservice/user/config"
	"github.com/koderover/zadig/v2/pkg/setting"
)

type Claims struct {
	Name              string          `json:"name"`
	Email             string          `json:"email"`
	Phone             string          `json:"phone"`
	UID               string          `json:"uid"`
	PreferredUsername string          `json:"preferred_username"`
	FederatedClaims   FederatedClaims `json:"federated_claims"`
	jwt.StandardClaims
}

type FederatedClaims struct {
	ConnectorId string `json:"connector_id"`
	UserId      string `json:"user_id"`
}

func CreateToken(claims *Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.SecretKey()))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func GetInternalToken(name string) (string, error) {
	claims := &Claims{
		Name:              name,
		UID:               "",
		Email:             fmt.Sprintf("%s@koderover.com", name),
		PreferredUsername: name,
		StandardClaims: jwt.StandardClaims{
			Audience:  setting.ProductName,
			ExpiresAt: 0,
		},
		FederatedClaims: FederatedClaims{
			ConnectorId: userconfig.SystemIdentityType,
			UserId:      name,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.SecretKey()))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
