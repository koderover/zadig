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
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/kube"
	e "github.com/koderover/zadig/pkg/tool/errors"
	"github.com/koderover/zadig/pkg/util"
)

var expirationTime = 10 * time.Hour

var awsKeyMap sync.Map

type awsKeyWithExpiration struct {
	AccessKey  string
	SecretKey  string
	Expiration int64
}

func (k *awsKeyWithExpiration) IsExpired() bool {
	if time.Now().Unix() > k.Expiration {
		return true
	}
	return false
}

func FindRegistryById(registryId string, getRealCredential bool, log *zap.SugaredLogger) (*models.RegistryNamespace, error) {
	return findRegisty(&mongodb.FindRegOps{ID: registryId}, getRealCredential, log)
}

func findRegisty(regOps *mongodb.FindRegOps, getRealCredential bool, log *zap.SugaredLogger) (*models.RegistryNamespace, error) {
	// TODO: 多租户适配
	resp, err := mongodb.NewRegistryNamespaceColl().Find(regOps)

	if err != nil {
		log.Warnf("RegistryNamespace.Find error: %s", err)
		resp = &models.RegistryNamespace{
			RegAddr:   config.RegistryAddress(),
			AccessKey: config.RegistryAccessKey(),
			SecretKey: config.RegistrySecretKey(),
			Namespace: config.RegistryNamespace(),
		}
	}

	if getRealCredential {
		switch resp.RegProvider {
		case config.RegistryTypeSWR:
			resp.AccessKey = fmt.Sprintf("%s@%s", resp.Region, resp.AccessKey)
			resp.SecretKey = util.ComputeHmacSha256(resp.AccessKey, resp.SecretKey)
		case config.RegistryTypeAWS:
			realAK, realSK, err := getAWSRegistryCredential(resp.ID.Hex(), resp.AccessKey, resp.SecretKey, resp.Region)
			if err != nil {
				log.Errorf("Failed to get keypair from aws, the error is: %s", err)
				return nil, err
			}
			resp.AccessKey = realAK
			resp.SecretKey = realSK
		default:
			break
		}
	}

	return resp, nil
}

func FindDefaultRegistry(getRealCredential bool, log *zap.SugaredLogger) (*models.RegistryNamespace, error) {
	return findRegisty(&mongodb.FindRegOps{IsDefault: true}, getRealCredential, log)
}

func ListRegistryNamespaces(getRealCredential bool, log *zap.SugaredLogger) ([]*models.RegistryNamespace, error) {
	resp, err := mongodb.NewRegistryNamespaceColl().FindAll(&mongodb.FindRegOps{})
	if err != nil {
		log.Errorf("RegistryNamespace.List error: %s", err)
		return resp, fmt.Errorf("RegistryNamespace.List error: %s", err)
	}

	if getRealCredential {
		for _, reg := range resp {
			switch reg.RegProvider {
			case config.RegistryTypeSWR:
				reg.AccessKey = fmt.Sprintf("%s@%s", reg.Region, reg.AccessKey)
				reg.SecretKey = util.ComputeHmacSha256(reg.AccessKey, reg.SecretKey)
			case config.RegistryTypeAWS:
				realAK, realSK, err := getAWSRegistryCredential(reg.ID.Hex(), reg.AccessKey, reg.SecretKey, reg.Region)
				if err != nil {
					log.Errorf("Failed to get keypair from aws, the error is: %s", err)
					return nil, err
				}
				reg.AccessKey = realAK
				reg.SecretKey = realSK
			default:
				break
			}
		}
	}

	return resp, nil
}

func EnsureDefaultRegistrySecret(namespace string, registryId string, kubeClient client.Client, log *zap.SugaredLogger) error {
	var reg *models.RegistryNamespace
	var err error
	if len(registryId) > 0 {
		reg, err = FindRegistryById(registryId, true, log)
		if err != nil {
			log.Errorf(
				"service.EnsureRegistrySecret: failed to find registry: %s error msg:%s",
				registryId, err,
			)
			return err
		}
	} else {
		reg, err = FindDefaultRegistry(true, log)
		if err != nil {
			log.Errorf(
				"service.EnsureRegistrySecret: failed to find default candidate registry: %s %s",
				namespace, err,
			)
			return err
		}
	}

	err = kube.CreateOrUpdateRegistrySecret(namespace, reg, kubeClient)
	if err != nil {
		log.Errorf("[%s] CreateDockerSecret error: %s", namespace, err)
		return e.ErrUpdateSecret.AddDesc(e.CreateDefaultRegistryErrMsg)
	}

	return nil
}

func getAWSRegistryCredential(ID, AK, SK, Region string) (string, string, error) {
	// first we try to get ak/sk from our memory cache
	obj, ok := awsKeyMap.Load(ID)
	if ok {
		keypair, ok := obj.(awsKeyWithExpiration)
		if ok {
			if !keypair.IsExpired() {
				fmt.Printf("Getting aws ak/sk from memory cache: ak[%s], sk[%s]", keypair.AccessKey, keypair.SecretKey)
				return keypair.AccessKey, keypair.SecretKey, nil
			}
		}
	}
	creds := credentials.NewStaticCredentials(AK, SK, "")
	config := &aws.Config{
		Region:      aws.String(Region),
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
		return "", "", errors.New("decode keypair from aws response error")
	}
	// cache the aws ak/sk
	awsKeyMap.Store(ID, awsKeyWithExpiration{
		AccessKey:  keypair[0],
		SecretKey:  keypair[1],
		Expiration: time.Now().Add(expirationTime).Unix(),
	})
	return keypair[0], keypair[1], nil
}
