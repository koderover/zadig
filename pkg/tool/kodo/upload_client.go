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

package kodo

import (
	"context"
	"fmt"

	"github.com/koderover/zadig/pkg/tool/kodo/qbox"
)

// UploadClient struct
type UploadClient struct {
	AccessKey string
	SecretKey string
	Bucket    string
}

// NewUploadClient constructor
func NewUploadClient(ak, sk, bucket string) (UploadClient, error) {

	cli := UploadClient{
		AccessKey: ak,
		SecretKey: sk,
		Bucket:    bucket,
	}

	if len(ak) == 0 || len(sk) == 0 || len(bucket) == 0 {
		return cli, fmt.Errorf("empty access key, secret key or bucket name")
	}

	return cli, nil
}

// UploadFile ...
func (cli UploadClient) UploadFile(key, localFile string) (retKey, retHash string, err error) {

	putPolicy := PutPolicy{
		Scope: fmt.Sprintf("%s:%s", cli.Bucket, key),
	}

	mac := qbox.NewMac(cli.AccessKey, cli.SecretKey)
	upToken := putPolicy.UploadToken(mac)
	cfg := Config{
		Zone:          &ZoneHuadong,
		UseHTTPS:      false,
		UseCdnDomains: false,
	}

	formUploader := NewFormUploader(&cfg)
	ret := PutRet{}
	err = formUploader.PutFile(context.Background(), &ret, upToken, key, localFile, nil)
	if err != nil {
		return
	}
	return ret.Key, ret.Hash, nil
}
