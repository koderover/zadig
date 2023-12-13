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
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/crypto"
)

type S3 struct {
	Ak        string `json:"ak"`
	Sk        string `json:"sk"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Subfolder string `json:"subfolder"`
	Insecure  bool   `json:"insecure"`
	IsDefault bool   `json:"is_default"`
	Provider  int    `json:"provider"`
	Region    string `json:"region"`
}

func (s *S3) GetSchema() string {
	if s.Insecure {
		return "http"
	}
	return "https"
}

func UnmarshalNewS3Storage(str string) (*S3, error) {
	s3 := new(S3)
	if err := json.Unmarshal([]byte(str), s3); err != nil {
		return nil, err
	}
	return s3, nil
}

func UnmarshalNewS3StorageFromEncrypted(encrypted, aesKey string) (*S3, error) {
	uri, err := crypto.AesDecrypt(encrypted, aesKey)
	if err != nil {
		return nil, err
	}

	return UnmarshalNewS3Storage(uri)
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
