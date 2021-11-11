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
	"fmt"
	"net/url"
	"strings"
)

// ParseOwnerAndRepo extracts the owner and repo info from the given link,
// the link must have to following format: http(s)://example.com/owner/repo
func ParseOwnerAndRepo(repoLink string) (string, string, error) {
	if !strings.HasPrefix(repoLink, "https://github.com") {
		return "", "", fmt.Errorf("invalid link, only github.com is supported")
	}
	uri, err := url.Parse(repoLink)
	if err != nil {
		return "", "", err
	}

	uriPath := strings.Trim(uri.Path, "/")
	ownerAndRepo := strings.Split(uriPath, "/")
	if len(ownerAndRepo) != 2 {
		return "", "", fmt.Errorf("can not find repo and owner in path %s", uriPath)
	}

	return ownerAndRepo[0], ownerAndRepo[1], nil
}
