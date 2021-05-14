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
)

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
	return strings.Join([]string{
		GetURLHostName(addr),
		namespace,
		parts[len(parts)-1],
	}, "/")
}

func GetUrl(URL string) (string, error) {
	uri, err := url.Parse(URL)
	if err != nil {
		return "", fmt.Errorf("url prase failed!")
	}
	return fmt.Sprintf("%s://%s", uri.Scheme, uri.Host), nil
}
