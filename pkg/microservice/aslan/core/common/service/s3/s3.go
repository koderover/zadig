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
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	awsS3 "github.com/aws/aws-sdk-go/service/s3"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/tool/crypto"
	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/fs"
)

type S3 struct {
	*models.S3Storage
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
func Validate(s *S3) error {
	s3Client, err := getAmazonClient(s)
	if err != nil {
		return err
	}
	listBucketInput := &awsS3.ListBucketsInput{}
	bucketListResp, err := s3Client.ListBuckets(listBucketInput)
	if err != nil {
		err = fmt.Errorf("validate S3 error: %s", err.Error())
		return err
	}
	for _, bucket := range bucketListResp.Buckets {
		if *bucket.Name == s.Bucket {
			return nil
		}
	}

	return fmt.Errorf("validate s3 error: given bucket does not exist")
}

// Download the file to object storage
func Download(ctx context.Context, storage *S3, src string, dest string) error {
	s3Client, err := getAmazonClient(storage)
	if err != nil {
		return err
	}

	retry := 0

	for retry < 3 {
		opt := &awsS3.GetObjectInput{
			Bucket: aws.String(storage.Bucket),
			Key:    aws.String(storage.GetObjectPath(src)),
		}
		obj, err := s3Client.GetObject(opt)
		if err != nil {
			retry++
			continue
		}
		err = fs.SaveFile(obj.Body, dest)
		if err == nil {
			return nil
		}
		retry++
	}
	return fmt.Errorf("下载文件%s/%s失败", storage.GetURI(), src)
}

// RemoveFiles will attempt to remove files specified in prefixList in `target` s3 endpoint-bucket recursively.
// If dryRun is given, the removal will be no op other than a log.
// Note that this API makes its best attempt to remove given files, if the removal fails, it will log error but the
// function just continues.
// CHANGED: DRY_RUN WILL NOT WORK NOW, THERE IS NO POINT OF DRY RUN.
func RemoveFiles(target *S3, prefixList []string, dryRun bool) {
	s3Client, err := getAmazonClient(target)
	if err != nil {
		log.Errorf("Fail to create minioClient for storage: %v; err: %v", target, err)
		return
	}
	deleteList := make([]*awsS3.ObjectIdentifier, 0)
	for _, prefix := range prefixList {
		input := &awsS3.ListObjectsInput{
			Bucket:    aws.String(target.Bucket),
			Delimiter: aws.String(""),
			Prefix:    aws.String(prefix),
		}
		objects, err := s3Client.ListObjects(input)
		if err != nil {
			log.Errorf("List s3 objects with prefix %s err: %+v", prefix, err)
			continue
		}
		for _, object := range objects.Contents {
			deleteList = append(deleteList, &awsS3.ObjectIdentifier{
				Key: object.Key,
			})
		}
	}

	input := &awsS3.DeleteObjectsInput{
		Bucket: aws.String(target.Bucket),
		Delete: &awsS3.Delete{Objects: deleteList},
	}
	_, err = s3Client.DeleteObjects(input)
	if err != nil {
		log.Errorf("Failed to delete object with prefix: %v from bucket %s", prefixList, target.Bucket)
	}
}

// Upload the file to object storage
func Upload(ctx context.Context, storage *S3, src string, dest string) error {
	s3Client, err := getAmazonClient(storage)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(src, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	// TODO: add md5 check for file integrity
	input := &awsS3.PutObjectInput{
		Body:   file,
		Bucket: aws.String(storage.Bucket),
		Key:    aws.String(dest),
	}
	_, err = s3Client.PutObject(input)
	return err
}

// ListFiles with specific prefix
func ListFiles(storage *S3, prefix string, recursive bool) ([]string, error) {
	ret := []string{}
	logger := log.SugaredLogger()

	s3Client, err := getAmazonClient(storage)
	if err != nil {
		return nil, err
	}

	input := &awsS3.ListObjectsInput{
		Bucket: aws.String(storage.Bucket),
		Prefix: aws.String(storage.GetObjectPath(prefix)),
	}
	if !recursive {
		input.Delimiter = aws.String("/")
	}
	output, err := s3Client.ListObjects(input)
	if err != nil {
		logger.Errorf("S3 [%v-%v] prefix [%v] listing objects failed: %v", storage.Endpoint, storage.Bucket, prefix, err)
		return nil, err
	}

	for _, item := range output.Contents {
		itemKey := *item.Key
		if strings.Index(itemKey, storage.GetObjectPath("")) == 0 {
			ret = append(ret, itemKey[len(storage.GetObjectPath("")):len(itemKey)])
		}
	}

	return ret, nil
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

func getAmazonClient(s *S3) (*awsS3.S3, error) {
	creds := credentials.NewStaticCredentials(s.Ak, s.Sk, "")
	endpoint := s.Endpoint
	config := &aws.Config{
		Endpoint:         &endpoint,
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      creds,
		DisableSSL:       &s.Insecure,
	}
	session, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}
	return awsS3.New(session), nil
}
