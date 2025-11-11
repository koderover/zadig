/*
Copyright 2024 The KodeRover Authors.

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

package util

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	registrytool "github.com/koderover/zadig/v2/pkg/tool/registries"
	"github.com/koderover/zadig/v2/pkg/util"
)

var awsKeyMap sync.Map
var expirationTime = 10 * time.Hour

type awsKeyWithExpiration struct {
	AccessKey  string
	SecretKey  string
	Expiration int64
}

func (k *awsKeyWithExpiration) IsExpired() bool {
	return time.Now().Unix() > k.Expiration
}

func DecodeRegistry(resp *models.RegistryNamespace) (*models.RegistryNamespace, error) {

	switch resp.RegProvider {
	case config.RegistryTypeSWR:
		resp.SecretKey = util.ComputeHmacSha256(resp.AccessKey, resp.SecretKey)
		resp.AccessKey = fmt.Sprintf("%s@%s", resp.Region, resp.AccessKey)
	case config.RegistryTypeAWS:
		realAK, realSK, err := GetAWSRegistryCredential(resp.ID.Hex(), resp.AccessKey, resp.SecretKey, resp.Region)
		if err != nil {
			log.Errorf("Failed to get keypair from aws, the error is: %s", err)
			return nil, err
		}
		resp.AccessKey = realAK
		resp.SecretKey = realSK
	}
	return resp, nil
}

func GetAWSRegistryCredential(id, ak, sk, region string) (realAK string, realSK string, err error) {
	// first we try to get ak/sk from our memory cache
	obj, ok := awsKeyMap.Load(id)
	if ok {
		keypair, ok := obj.(awsKeyWithExpiration)
		if ok {
			if !keypair.IsExpired() {
				return keypair.AccessKey, keypair.SecretKey, nil
			}
		}
	}
	creds := credentials.NewStaticCredentials(ak, sk, "")
	config := &aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	}
	sess, err := session.NewSession(config)
	if err != nil {
		return "", "", err
	}
	svc := ecr.New(sess)
	input := &ecr.GetAuthorizationTokenInput{}

	result, err := svc.GetAuthorizationToken(input)
	if err != nil {
		return "", "", err
	}
	// since the new AWS ECR will give a token that has access to ALL the repository, we use the first token
	encodedToken := *result.AuthorizationData[0].AuthorizationToken
	rawDecodedText, err := base64.StdEncoding.DecodeString(encodedToken)
	if err != nil {
		return "", "", err
	}
	keypair := strings.Split(string(rawDecodedText), ":")
	if len(keypair) != 2 {
		return "", "", errors.New("format of keypair is invalid")
	}
	// cache the aws ak/sk
	awsKeyMap.Store(id, awsKeyWithExpiration{
		AccessKey:  keypair[0],
		SecretKey:  keypair[1],
		Expiration: time.Now().Add(expirationTime).Unix(),
	})
	return keypair[0], keypair[1], nil
}

func SyncDinDForRegistries() error {
	registries, err := mongodb.NewRegistryNamespaceColl().FindAll(&mongodb.FindRegOps{})
	if err != nil {
		return fmt.Errorf("failed to list registry to update dind, err: %s", err)
	}

	regList := make([]*registrytool.RegistryInfoForDinDUpdate, 0)
	for _, reg := range registries {
		regItem := &registrytool.RegistryInfoForDinDUpdate{
			ID:      reg.ID,
			RegAddr: reg.RegAddr,
		}
		if reg.AdvancedSetting != nil {
			regItem.AdvancedSetting = &registrytool.RegistryAdvancedSetting{
				TLSEnabled: reg.AdvancedSetting.TLSEnabled,
				TLSCert:    reg.AdvancedSetting.TLSCert,
			}
		}
		regList = append(regList, regItem)
	}

	dynamicClient, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(setting.LocalClusterID)
	if err != nil {
		return fmt.Errorf("failed to get dynamic client to update dind, err: %s", err)
	}

	// Get storage driver from cluster config
	storageDriver := ""
	cluster, err := mongodb.NewK8SClusterColl().Get(setting.LocalClusterID)
	if err == nil && cluster.DindCfg != nil {
		storageDriver = cluster.DindCfg.StorageDriver
	}

	return registrytool.PrepareDinD(dynamicClient, config.Namespace(), regList, storageDriver)
}
