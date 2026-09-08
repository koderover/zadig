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

package permission

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"regexp"
	"sync"
	"time"

	globalConfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/cache"
)

const (
	ClusterAgentDownloadTokenQuery = "download_token"
	ClusterAgentDownloadTokenTTL   = 5 * time.Minute
	clusterAgentDownloadTokenKey   = "cluster-agent-yaml-token:"
)

var clusterAgentYamlURLRegexp = regexp.MustCompile(`^/api/aslan/cluster/agent/([\w-]+)/agent\.yaml$`)

var clusterAgentDownloadTokenCache = sync.OnceValue(func() *cache.RedisCache {
	return cache.NewIsolatedRedisCache(globalConfig.RedisCommonCacheTokenDB())
})

func ClusterIDFromAgentYamlPath(path string) (string, bool) {
	match := clusterAgentYamlURLRegexp.FindStringSubmatch(path)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func NewClusterAgentDownloadToken(clusterID string) (string, int64, error) {
	expiresAt := time.Now().Add(ClusterAgentDownloadTokenTTL)
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", 0, err
	}

	token := base64.RawURLEncoding.EncodeToString(random)
	err := clusterAgentDownloadTokenCache().Write(clusterAgentDownloadTokenCacheKey(token), clusterID, ClusterAgentDownloadTokenTTL)
	if err != nil {
		return "", 0, err
	}
	return token, expiresAt.Unix(), nil
}

func ValidateClusterAgentDownloadToken(clusterID, token string) (bool, error) {
	if clusterID == "" || token == "" {
		return false, nil
	}
	storedClusterID, err := clusterAgentDownloadTokenCache().GetString(clusterAgentDownloadTokenCacheKey(token))
	if err != nil {
		return false, err
	}
	return storedClusterID == clusterID, nil
}

func ConsumeClusterAgentDownloadToken(clusterID, token string) (bool, error) {
	if clusterID == "" || token == "" {
		return false, nil
	}
	return clusterAgentDownloadTokenCache().CompareAndDelete(clusterAgentDownloadTokenCacheKey(token), clusterID)
}

func clusterAgentDownloadTokenCacheKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return clusterAgentDownloadTokenKey + hex.EncodeToString(digest[:])
}
