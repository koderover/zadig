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
	"encoding/base64"
	"net/url"
	"strings"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/setting"
	kubeclient "github.com/koderover/zadig/pkg/shared/kube/client"
	"github.com/koderover/zadig/pkg/tool/rsa"
)

func GetRSAKey() ([]byte, []byte, error) {
	clientset, err := kubeclient.GetKubeClientSet(config.HubServerAddress(), setting.LocalClusterID)
	if err != nil {
		return nil, nil, err
	}
	res, err := clientset.CoreV1().Secrets(config.Namespace()).Get(context.TODO(), setting.RSASecretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	return res.Data["publicKey"], res.Data["privateKey"], nil
}

type GetRSAPublicKeyRes struct {
	PublicKey string `json:"publicKey"`
}

func GetRSAPublicKey() (*GetRSAPublicKeyRes, error) {
	publicKey, _, err := GetRSAKey()
	if err != nil {
		return nil, err
	}
	return &GetRSAPublicKeyRes{
		PublicKey: string(publicKey),
	}, nil
}

type GetAesKeyFromEncryptedKeyResp struct {
	PlainText string `json:"plain_text"`
}

func GetAesKeyFromEncryptedKey(encryptedKey string, log *zap.SugaredLogger) (*GetAesKeyFromEncryptedKeyResp, error) {
	_, privateKey, err := GetRSAKey()
	if err != nil {
		log.Errorf("getAesKeyFromEncryptedKey getRSAKey error msg:%s", err)
		return nil, err
	}
	decodedKey, err := url.QueryUnescape(encryptedKey)
	if err != nil {
		log.Errorf("GetAesKeyFromEncryptedKey QueryUnescape error:%s", err)
		return nil, err
	}
	decodedKey = strings.ReplaceAll(decodedKey, " ", "+")
	byteKey, err := base64.StdEncoding.DecodeString(decodedKey)
	if err != nil {
		log.Errorf("getAesKeyFromEncryptedKey decodeString error msg:%s", err)
		return nil, err
	}

	aesKey, err := rsa.DecryptByPrivateKey(byteKey, privateKey)
	if err != nil {
		log.Errorf("getAesKeyFromEncryptedKey decryptByPrivateKey error msg:%s", err)
		return nil, err
	}

	return &GetAesKeyFromEncryptedKeyResp{
		PlainText: string(aesKey),
	}, nil
}
