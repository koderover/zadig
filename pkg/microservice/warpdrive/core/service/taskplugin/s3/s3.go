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
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	config2 "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/crypto"
)

type S3 struct {
	*Storage
}

type Storage struct {
	//ID          primitive.ObjectID `bson:"_id"         json:"id"`
	Ak          string `bson:"ak"          json:"ak"`
	Sk          string `bson:"-"           json:"sk"`
	Endpoint    string `bson:"endpoint"    json:"endpoint"`
	Bucket      string `bson:"bucket"      json:"bucket"`
	Subfolder   string `bson:"subfolder"   json:"subfolder"`
	Insecure    bool   `bson:"insecure"    json:"insecure"`
	IsDefault   bool   `bson:"is_default"  json:"is_default"`
	EncryptedSk string `bson:"encryptedSk" json:"-"`
	UpdatedBy   string `bson:"updated_by"  json:"updated_by"`
	UpdateTime  int64  `bson:"update_time" json:"update_time"`
	Provider    int8   `bson:"provider"    json:"provider"`
}

func (s *S3) GetSchema() string {
	if s.Insecure {
		return "http"
	}
	return "https"
}

func (s *S3) GetEncryptedURL() (encrypted string, err error) {
	return crypto.AesEncrypt(s.GetURL())
}

func (s *S3) GetURL() string {
	return strings.TrimRight(
		fmt.Sprintf(
			"%s://%s:%s@%s/%s/%s", s.GetSchema(), s.Ak, s.Sk, s.Endpoint, s.Bucket, s.Subfolder,
		),
		"/",
	)
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
		&Storage{
			Ak:        store.User.Username(),
			Sk:        sk,
			Endpoint:  store.Host,
			Bucket:    bucket,
			Subfolder: subfolder,
			Insecure:  store.Scheme == "http",
		},
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
