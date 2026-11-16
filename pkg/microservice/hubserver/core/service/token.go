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
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/clientmanager"
	"github.com/koderover/zadig/v2/pkg/tool/crypto"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

const (
	// Retain the previous key only during rotation so old agents can reconnect and be upgraded.
	previousAESKeySecretName  = "zadig-aes-key-previous"
	previousAESKeyCacheTTL    = 30 * time.Second
	previousAESKeyReadTimeout = 5 * time.Second
)

var previousAESKey struct {
	sync.Mutex
	key       string
	expiresAt time.Time
}

// DecryptClusterToken decrypts the token presented by a hub agent into its cluster ID.
// Tokens encrypted with the previous encryption key are accepted while secret zadig-aes-key-previous exists.
// AES-CFB has no integrity check, a wrong key yields garbage instead of an error, so the result must be a valid cluster ID.
func DecryptClusterToken(token string) (string, error) {
	clusterID, err := crypto.AesDecrypt(token)
	if err == nil && primitive.IsValidObjectID(clusterID) {
		return clusterID, nil
	}

	if key := getPreviousAESKey(); key != "" {
		clusterID, err = crypto.AesDecrypt(token, key)
		if err == nil && primitive.IsValidObjectID(clusterID) {
			return clusterID, nil
		}
	}

	return "", errors.New("invalid cluster token")
}

// getPreviousAESKey returns the previous encryption key, or an empty string if it does not exist or cannot be read.
func getPreviousAESKey() string {
	previousAESKey.Lock()
	defer previousAESKey.Unlock()

	if time.Now().Before(previousAESKey.expiresAt) {
		return previousAESKey.key
	}

	previousAESKey.key = ""
	previousAESKey.expiresAt = time.Now().Add(previousAESKeyCacheTTL)

	clientset, err := clientmanager.NewKubeClientManager().GetKubernetesClientSet(setting.LocalClusterID)
	if err != nil {
		log.Warnf("failed to get kube client to read secret %s: %s", previousAESKeySecretName, err)
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), previousAESKeyReadTimeout)
	defer cancel()
	secret, err := clientset.CoreV1().Secrets(config.Namespace()).Get(ctx, previousAESKeySecretName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warnf("failed to read secret %s: %s", previousAESKeySecretName, err)
		}
		return ""
	}

	previousAESKey.key = strings.TrimSpace(string(secret.Data["aesKey"]))
	return previousAESKey.key
}
