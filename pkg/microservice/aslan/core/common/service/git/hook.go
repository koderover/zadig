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

package git

import (
	"sync"

	"github.com/koderover/zadig/pkg/tool/crypto"
)

var once sync.Once
var secret string

func GetHookSecret() string {
	once.Do(func() {
		//poetryClient := poetry.New(config.PoetryAPIServer())
		//org, err := poetryClient.GetOrganization(poetry.DefaultOrganization)
		//if err != nil {
		//	log.Errorf("failed to find default organization: %v", err)
		//	secret = "--impossible-token--"
		//}
		token, err := crypto.AesEncrypt("hook")
		if err != nil {
			panic("Failed to get token")
		}
		secret = token
	})

	return secret
}
