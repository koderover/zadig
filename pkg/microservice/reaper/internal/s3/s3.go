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
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/minio/minio-go"

	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/log"
)

type S3 struct {
	Ak        string `json:"ak"`
	Sk        string `json:"sk"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	Subfolder string `json:"subfolder"`
	Insecure  bool   `json:"insecure"`
	IsDefault bool   `json:"is_default"`
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

	return &S3{
		Ak:        store.User.Username(),
		Sk:        sk,
		Endpoint:  store.Host,
		Bucket:    bucket,
		Subfolder: subfolder,
		Insecure:  store.Scheme == "http",
	}, nil
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

// Validate the existence of bucket

func ReaperDownload(ctx context.Context, storage *S3, src string, dest string) (err error) {
	var minioClient *minio.Client
	if minioClient, err = getMinioClient(storage); err != nil {
		return
	}

	retry := 0

	for retry < 3 {
		err = minioClient.FGetObjectWithContext(
			ctx, storage.Bucket, storage.GetObjectPath(src), dest, minio.GetObjectOptions{},
		)
		if err == nil {
			return
		}

		if strings.Contains(err.Error(), "stream error") {
			retry++
			continue
		}
		// 自定义返回的错误信息
		err = fmt.Errorf("未找到 package %s/%s，请确认打包脚本是否正确。", storage.GetURI(), src)
		return
	}

	return
}

// Download the file to object storage
func Download(ctx context.Context, storage *S3, src string, dest string) (err error) {
	log := log.SugaredLogger()
	var minioClient *minio.Client
	if minioClient, err = getMinioClient(storage); err != nil {
		return
	}

	retry := 0

	for retry < 3 {
		err = minioClient.FGetObjectWithContext(
			ctx, storage.Bucket, storage.GetObjectPath(src), dest, minio.GetObjectOptions{},
		)

		if err == nil {
			return
		}
		log.Warnf("failed to download file %s %s=>%s: %v", storage.GetURI(), src, dest, err)

		if strings.Contains(err.Error(), "stream error") {
			retry++
			continue
		}
		// 自定义返回的错误信息
		err = fmt.Errorf("未找到 package %s/%s，请确认打包脚本是否正确。", storage.GetURI(), src)
		return
	}

	return
}

// Upload the file to object storage
func Upload(ctx context.Context, storage *S3, src string, dest string) (err error) {
	var minioClient *minio.Client
	if minioClient, err = getMinioClient(storage); err != nil {
		return
	}

	_, err = minioClient.FPutObjectWithContext(
		ctx, storage.Bucket, storage.GetObjectPath(dest), src, minio.PutObjectOptions{},
	)

	return
}

// ListFiles with specific prefix
func ListFiles(storage *S3, prefix string, recursive bool) (files []string, err error) {
	log := log.SugaredLogger()
	var minioClient *minio.Client
	if minioClient, err = getMinioClient(storage); err != nil {
		return
	}

	done := make(chan struct{})
	defer close(done)

	for item := range minioClient.ListObjects(storage.Bucket, storage.GetObjectPath(prefix), recursive, done) {
		if item.Err != nil {
			err = multierror.Append(err, item.Err)
			continue
		}
		key := item.Key
		if strings.Index(key, storage.GetObjectPath("")) == 0 {
			files = append(files, item.Key[len(storage.GetObjectPath("")):len(item.Key)])
		}
	}
	if err != nil {
		log.Errorf("S3 [%v-%v] prefix [%v] listing objects failed: %v", storage.Endpoint, storage.Bucket, prefix, err)
	}

	return
}

func getMinioClient(s *S3) (minioClient *minio.Client, err error) {
	if minioClient, err = minio.New(s.Endpoint, s.Ak, s.Sk, !s.Insecure); err != nil {
		return
	}

	return
}
