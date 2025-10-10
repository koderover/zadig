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
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	fsutil "github.com/koderover/zadig/v2/pkg/util/fs"
)

const (
	DefaultRegion = "ap-shanghai"
)

type Client struct {
	*s3.S3
}

type DownloadOption struct {
	IgnoreNotExistError bool
	RetryNum            int
}

var defaultDownloadOption = &DownloadOption{
	RetryNum: 3,
}

func NewClient(endpoint, ak, sk, region string, insecure bool, provider int8) (*Client, error) {
	s3ForcePathStyle := true
	if provider == setting.ProviderSourceAli || provider == setting.ProviderSourceTencent || provider == setting.ProviderSourceHuawei || provider == setting.ProviderSourceVolcano {
		s3ForcePathStyle = false
	}

	creds := credentials.NewStaticCredentials(ak, sk, "")
	config := &aws.Config{
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(s3ForcePathStyle),
		Credentials:      creds,
		DisableSSL:       aws.Bool(insecure),
	}
	if region != "" {
		config.Region = aws.String(region)
	} else {
		config.Region = aws.String(DefaultRegion)
	}
	session, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}
	return &Client{s3.New(session)}, nil
}

// Validate the existence of bucket
func (c *Client) ValidateBucket(bucketName string) error {
	headBucketInput := &s3.HeadBucketInput{Bucket: aws.String(bucketName)}
	_, err := c.HeadBucket(headBucketInput)
	if err != nil {
		return fmt.Errorf("validate S3 error: %s", err.Error())
	}

	return nil
}

func (c *Client) DownloadWithOption(bucketName, objectKey, dest string, option *DownloadOption) error {
	return c.download(bucketName, objectKey, dest, option)
}

// Download the file to object storage
func (c *Client) Download(bucketName, objectKey, dest string) error {
	return c.download(bucketName, objectKey, dest, defaultDownloadOption)
}

func (c *Client) download(bucketName, objectKey, dest string, option *DownloadOption) error {

	retry := 0
	var err error

	for retry < option.RetryNum {
		opt := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		}
		obj, err1 := c.GetObject(opt)
		if err1 != nil {
			if e, ok := err1.(awserr.Error); ok && e.Code() == s3.ErrCodeNoSuchKey {
				if option.IgnoreNotExistError {
					return nil
				} else {
					return fmt.Errorf("object %s not found in bucket %s, err: %v", objectKey, bucketName, err1)
				}
			}

			log.Warnf("Failed to get object %s from s3, try again, err: %s", objectKey, err1)
			err = err1

			retry++
			continue
		}
		err = fsutil.SaveFile(obj.Body, dest)
		if err != nil {
			log.Errorf("Failed to save file to %s, err: %s", dest, err)
		}
		return err
	}

	return err
}

// CopyObject copies an object to a new place in the same bucket.
func (c *Client) CopyObject(bucketName, oldKey, newKey string) error {
	opt := &s3.CopyObjectInput{
		Bucket:     aws.String(bucketName),
		CopySource: aws.String(bucketName + "/" + oldKey),
		Key:        aws.String(newKey),
	}
	_, err := c.S3.CopyObject(opt)

	return err
}

// DeleteObjects deletes all the objects listed in keys.
func (c *Client) DeleteObjects(bucketName string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	var ids []*s3.ObjectIdentifier
	for _, k := range keys {
		ids = append(ids, &s3.ObjectIdentifier{
			Key: aws.String(k),
		})
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &s3.Delete{Objects: ids},
	}

	_, err := c.S3.DeleteObjects(input)

	return err
}

// RemoveFiles removes the files with a specific list of prefixes and delete ALL of them
// for NOW, if an error is encountered, nothing will happen except for a line of error log.
func (c *Client) RemoveFiles(bucketName string, prefixList []string) {
	deleteList := make([]*s3.ObjectIdentifier, 0)
	for _, prefix := range prefixList {
		input := &s3.ListObjectsInput{
			Bucket:    aws.String(bucketName),
			Delimiter: aws.String(""),
			Prefix:    aws.String(prefix),
		}
		objects, err := c.ListObjects(input)
		if err != nil {
			log.Errorf("Failed to list s3 objects with prefix %s err: %s", prefix, err)
			continue
		}
		for _, object := range objects.Contents {
			deleteList = append(deleteList, &s3.ObjectIdentifier{
				Key: object.Key,
			})
		}
	}

	if len(deleteList) == 0 {
		log.Warnf("Nothing to remove")
		return
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &s3.Delete{Objects: deleteList},
	}

	_, err := c.S3.DeleteObjects(input)
	if err != nil {
		log.Errorf("Failed to delete object with prefix: %v in bucket %s, err: %s", prefixList, bucketName, err)
	}
}

func detectMimetype(path string) string {
	fileext := filepath.Ext(path)
	if fileext == "" {
		return ""
	}
	return mime.TypeByExtension(fileext)
}

// Upload uploads a file from src to the bucket with the specified objectKey
func (c *Client) Upload(bucketName, src string, objectKey string) error {
	file, err := os.OpenFile(src, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// TODO: add md5 check for file integrity
	input := &s3.PutObjectInput{
		Body:   file,
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}
	mimetype := detectMimetype(src)
	if mimetype != "" {
		input.ContentType = &mimetype
	}
	_, err = c.PutObject(input)
	return err
}

// Upload upload all files in a directory to a S3 path recursively
func (c *Client) UploadDir(bucketName, srcdir string, s3dir string) error {
	err := fs.WalkDir(os.DirFS(srcdir), ".", func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		key := path.Join(s3dir, p)
		originalFilePath := filepath.Join(srcdir, p)
		return c.Upload(bucketName, originalFilePath, key)
	})
	return err
}

func (c *Client) GetFile(bucketName, objectKey string, option *DownloadOption) (*s3.GetObjectOutput, error) {
	retry := 0
	var err error

	for retry < option.RetryNum {
		opt := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		}
		obj, err1 := c.GetObject(opt)
		if err1 != nil {
			if e, ok := err1.(awserr.Error); ok && e.Code() == s3.ErrCodeNoSuchKey {
				if option.IgnoreNotExistError {
					return nil, nil
				}
				return nil, err1
			}

			log.Warnf("Failed to get object %s from s3, try again, err: %s", objectKey, err1)
			err = err1
			retry++
			continue
		}

		return obj, nil
	}
	return nil, err
}

// ListFiles with given prefix
func (c *Client) ListFiles(bucketName, prefix string, recursive bool) ([]string, error) {
	ret := make([]string, 0)

	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	}
	if !recursive {
		input.Delimiter = aws.String("/")
	}
	output, err := c.ListObjects(input)
	if err != nil {
		log.Errorf("bucket [%s] listing objects with prefix [%v] failed, error: %v", bucketName, prefix, err)
		return nil, err
	}

	for _, item := range output.Contents {
		itemKey := *item.Key
		ret = append(ret, itemKey)
	}

	return ret, nil
}
