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

package util

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/types"
)

func TrimURLScheme(urlAddr string) string {
	uri, err := url.Parse(strings.TrimSuffix(urlAddr, "/"))
	if err != nil {
		return urlAddr
	}
	return uri.Host + uri.Path
}

// GetURLHostName ...
func GetURLHostName(urlAddr string) string {

	uri, err := url.Parse(urlAddr)
	if err != nil {
		return urlAddr
	}
	return uri.Host
}

func ReplaceRepo(origin, addr, namespace string) string {
	parts := strings.SplitN(origin, "/", -1)
	if namespace != "" {
		return strings.Join([]string{
			TrimURLScheme(addr),
			namespace,
			parts[len(parts)-1],
		}, "/")
	}
	return strings.Join([]string{
		TrimURLScheme(addr),
		parts[len(parts)-1],
	}, "/")
}

func ReplaceTag(origin, newTag string) string {
	parts := strings.Split(origin, ":")
	if len(parts) != 2 {
		return origin
	}
	parts[len(parts)-1] = newTag
	return strings.Join(parts, ":")
}

func GetAddress(uri string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("url prase failed: %s", err)
	}
	if u.Scheme == "" {
		return "", fmt.Errorf("scheme is missing")
	}

	return fmt.Sprintf("%s://%s", u.Scheme, u.Host), nil
}

// ParseOwnerAndRepo extracts the owner and repo info from the given link,
// the link must have to following format: http(s)://example.com/owner/repo
func ParseOwnerAndRepo(repoLink string, authType types.AuthType) (string, string) {
	var ownerAndRepo []string
	repoLink = strings.TrimSuffix(repoLink, ".git")
	switch authType {
	case types.SSHAuthType:
		repoLinkArr := strings.Split(repoLink, ":")
		currentRepoLink := repoLinkArr[len(repoLinkArr)-1]
		uriPath := strings.Trim(currentRepoLink, "/")
		ownerAndRepo = strings.Split(uriPath, "/")
	case types.PrivateAccessTokenAuthType:
		uri, err := url.Parse(repoLink)
		if err != nil {
			log.Errorf("failed to parse url, err:%s", err)
			return "", ""
		}

		uriPath := strings.Trim(uri.Path, "/")
		ownerAndRepo = strings.Split(uriPath, "/")
	}
	if len(ownerAndRepo) != 2 {
		return "", ""
	}

	return ownerAndRepo[0], ownerAndRepo[1]
}
