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
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/minio/minio-go"

	"github.com/koderover/zadig/lib/microservice/aslan/config"
	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	"github.com/koderover/zadig/lib/tool/crypto"
	"github.com/koderover/zadig/lib/tool/xlog"
)

type S3 struct {
	*models.S3Storage
}

func (s *S3) GetSchema() string {
	if s.Insecure {
		return "http"
	} else {
		return "https"
	}
}

func (s *S3) GetEncryptedUrl() (encrypted string, err error) {
	return crypto.AesEncrypt(s.GetUrl())
}

func (s *S3) GetUrl() string {
	return strings.TrimRight(
		fmt.Sprintf(
			"%s://%s:%s@%s/%s/%s", s.GetSchema(), s.Ak, s.Sk, s.Endpoint, s.Bucket, s.Subfolder,
		),
		"/",
	)
}

func NewS3StorageFromUrl(uri string) (*S3, error) {
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
		&models.S3Storage{
			Ak:        store.User.Username(),
			Sk:        sk,
			Endpoint:  store.Host,
			Bucket:    bucket,
			Subfolder: subfolder,
			Insecure:  store.Scheme == "http",
		},
	}, nil
}

func NewS3StorageFromEncryptedUri(encryptedUri, key string) (*S3, error) {
	var uri string

	if key != "" {
		if aes, err := crypto.NewAes(key); err != nil {
			return nil, err
		} else {
			if uri, err = aes.Decrypt(encryptedUri); err != nil {
				return nil, err
			}
		}
	} else {
		uri = encryptedUri
	}

	return NewS3StorageFromUrl(uri)
}

func (s *S3) GetUri() string {
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
func Validate(s *S3) (err error) {
	var minioClient *minio.Client
	if minioClient, err = getMinioClient(s); err != nil {
		return
	}

	var exists bool

	if exists, err = minioClient.BucketExists(s.Bucket); err != nil {
		return
	} else if !exists {
		err = fmt.Errorf("no bucket named %s", s.Bucket)
		return
	}

	return
}

// Download the file to object storage
func Download(ctx context.Context, storage *S3, src string, dest string) (err error) {
	log := xlog.NewDummy()
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
		} else {
			log.Warnf("failed to download file %s %s=>%s: %v", storage.GetUri(), src, dest, err)
			// 自定义返回的错误信息
			err = fmt.Errorf("未找到 package %s/%s，请确认打包脚本是否正确。", storage.GetUri(), src)
			if strings.Contains(err.Error(), "stream error") {
				retry += 1
				continue
			} else {
				return
			}
		}
	}

	return
}

// RemoveFiles will attempt to remove files specified in prefixList in `target` s3 endpoint-bucket recursively.
// If dryRun is given, the removal will be no op other than a log.
// Note that this API makes its best attempt to remove given files, if the removal fails, it will log error but the
// function just continues.
func RemoveFiles(target *S3, prefixList []string, dryRun bool) {
	log := xlog.NewDummy()
	minioClient, err := getMinioClient(target)
	if err != nil {
		log.Errorf("Fail to create minioClient for storage: %v; err: %v", target, err)
		return
	}

	objectsCh := make(chan string)
	var wg sync.WaitGroup
	var cnt int64
	for _, prefix := range prefixList {
		wg.Add(1)
		go func(filePrefix string) {
			defer wg.Done()
			objects, err := ListFiles(target, filePrefix, true /* recursive */)
			if err != nil {
				// Failing to remove some S3 files is not fatal, log and move on to try next batch
				log.Errorf("Error detected while listing files to be removed for [%v-%v-%v] with error: %v",
					target.Endpoint, target.Bucket, filePrefix, err)
				return
			}
			atomic.AddInt64(&cnt, int64(len(objects)))
			for _, o := range objects {
				o = filepath.Join(target.GetObjectPath(""), o)
				if !dryRun {
					objectsCh <- o
				} else {
					log.Infof("%v-%v removing file [dryRun %v]: %v",
						target.Endpoint, target.Bucket, dryRun, o)
				}
			}
		}(prefix)
	}
	errCh := minioClient.RemoveObjects(target.Bucket, objectsCh)
	if waitTimeout(&wg, 15*time.Minute) {
		log.Errorf("%v-%v removing files timeout waiting for all groups [dryRun %v]", target.Endpoint, target.Bucket, dryRun)
	}
	close(objectsCh)
	log.Infof("%v-%v removing %d files [dryRun %v]", target.Endpoint, target.Bucket, cnt, dryRun)
	for err := range errCh {
		log.Errorf("%v-%v removing file failed with error: %v", target.Endpoint, target.Bucket, err)
	}
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
	log := xlog.NewDummy()
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

func FindDefaultS3() (*S3, error) {
	storage, err := commonrepo.NewS3StorageColl().FindDefault()
	if err != nil {
		return &S3{
			S3Storage: &models.S3Storage{
				Ak:       config.S3StorageAK(),
				Sk:       config.S3StorageSK(),
				Endpoint: config.S3StorageEndpoint(),
				Bucket:   config.S3StorageBucket(),
				Insecure: config.S3StorageProtocol() == "http",
			},
		}, nil
	}

	return &S3{S3Storage: storage}, nil
}

// 获取内置的s3
func FindInternalS3() *S3 {
	storage := &models.S3Storage{
		Ak:       config.S3StorageAK(),
		Sk:       config.S3StorageSK(),
		Endpoint: config.S3StorageEndpoint(),
		Bucket:   config.S3StorageBucket(),
		Insecure: config.S3StorageProtocol() == "http",
	}
	return &S3{S3Storage: storage}
}

func getMinioClient(s *S3) (minioClient *minio.Client, err error) {
	if minioClient, err = minio.New(s.Endpoint, s.Ak, s.Sk, !s.Insecure); err != nil {
		return
	}

	return
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	normal := make(chan struct{})
	go func() {
		defer close(normal)
		wg.Wait()
	}()
	select {
	case <-normal:
		return false
	case <-time.After(timeout):
		return true
	}
}
