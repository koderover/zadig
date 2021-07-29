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

package s3

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

type S3 struct {
	Ak        string `json:"ak"`
	Sk        string `json:"sk"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Subfolder string `json:"subfolder"`
	Insecure  bool   `json:"insecure"`
	IsDefault bool   `json:"is_default"`
	Provider  int8   `json:"provider"`
}

func (s *S3) GetSchema() string {
	if s.Insecure {
		return "http"
	}
	return "https"
}

func NewS3StorageFromURL(uri string) (*S3, error) {
	store, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	sk, _ := store.User.Password()
	paths := strings.Split(strings.TrimLeft(store.Path, "/"), "/")
	bucket := paths[0]

	var subfolder string
	if len(paths) > 1 {
		subfolder = strings.Join(paths[1:], "/")
	}

	ret := &S3{
		Ak:        store.User.Username(),
		Sk:        sk,
		Endpoint:  store.Host,
		Bucket:    bucket,
		Subfolder: subfolder,
		Insecure:  store.Scheme == "http",
	}
	if strings.Contains(store.Host, config2.MinioServiceName()) {
		ret.Provider = setting.ProviderSourceSystemDefault
	}

	return ret, nil
}

func NewS3StorageFromEncryptedURI(encryptedURI string) (*S3, error) {
	uri, err := crypto.AesDecrypt(encryptedURI)
	if err != nil {
		return nil, err
	}

	return NewS3StorageFromURL(uri)
}

func (s *S3) GetURI() string {
	return strings.TrimRight(
		fmt.Sprintf(
			"%s://%s/%s/%s", s.GetSchema(), s.Endpoint, s.Bucket, s.Subfolder,
		),
		"/",
	)
}

func (s *S3) GetObjectPath(name string) string {
	// target should not be started with /
	if s.Subfolder != "" {
		return strings.TrimLeft(filepath.Join(s.Subfolder, name), "/")
	}

	return strings.TrimLeft(name, "/")
}

func (s *S3) Validate() error {
	s.Ak = strings.Trim(s.Ak, " ")
	s.Sk = strings.Trim(s.Sk, " ")
	s.Bucket = strings.Trim(s.Bucket, " /")
	s.Subfolder = strings.Trim(s.Subfolder, " /")
	s.Endpoint = strings.Trim(s.Endpoint, " /")

	if s.Ak == "" || s.Sk == "" || s.Bucket == "" || s.Endpoint == "" {
		return errors.New("required field is missing")
	}

	return nil
}
