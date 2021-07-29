package s3

import (
	"fmt"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/koderover/zadig/pkg/tool/log"
	"github.com/koderover/zadig/pkg/util/fs"
)

const (
	DefaultRegion = "ap-shanghai"
)

type Client struct {
	*s3.S3
}

func NewClient(endpoint, ak, sk string, insecure, forcedPathStyle bool) (*Client, error) {
	creds := credentials.NewStaticCredentials(ak, sk, "")
	config := &aws.Config{
		Region:           aws.String(DefaultRegion),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(forcedPathStyle),
		Credentials:      creds,
		DisableSSL:       aws.Bool(insecure),
	}
	session, err := session.NewSession(config)
	if err != nil {
		return nil, err
	}
	return &Client{s3.New(session)}, nil
}

// Validate the existence of bucket
func (c *Client) ValidateBucket(bucketName string) error {
	listBucketInput := &s3.ListBucketsInput{}
	bucketListResp, err := c.ListBuckets(listBucketInput)
	if err != nil {
		return fmt.Errorf("validate S3 error: %s", err.Error())
	}
	for _, bucket := range bucketListResp.Buckets {
		if *bucket.Name == bucketName {
			return nil
		}
	}

	return fmt.Errorf("validate s3 error: given bucket does not exist")
}

// Download the file to object storage
func (c *Client) Download(bucketName, objectKey, dest string) error {
	retry := 0

	for retry < 3 {
		opt := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
		}
		obj, err := c.GetObject(opt)
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
	return fmt.Errorf("download file with key: %s failed", objectKey)
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
			log.Errorf("List s3 objects with prefix %s err: %+v", prefix, err)
			continue
		}
		for _, object := range objects.Contents {
			deleteList = append(deleteList, &s3.ObjectIdentifier{
				Key: object.Key,
			})
		}
	}

	input := &s3.DeleteObjectsInput{
		Bucket: aws.String(bucketName),
		Delete: &s3.Delete{Objects: deleteList},
	}
	_, err := c.DeleteObjects(input)
	if err != nil {
		log.Errorf("Failed to delete object with prefix: %v from bucket %s", prefixList, bucketName)
	}
}

// Upload uploads a file from src to the bucket with the specified objectKey
func (c *Client) Upload(bucketName, src string, objectKey string) error {
	file, err := os.OpenFile(src, os.O_RDONLY, 0600)
	if err != nil {
		return err
	}
	// TODO: add md5 check for file integrity
	input := &s3.PutObjectInput{
		Body:   file,
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	}
	_, err = c.PutObject(input)
	return err
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
		_, fileName := path.Split(itemKey)
		ret = append(ret, fileName)
	}

	return ret, nil
}
