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
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/koderover/zadig/pkg/tool/kodo/qbox"
)

// PutPolicy 表示文件上传的上传策略
type PutPolicy struct {
	Scope               string `json:"scope"`
	Expires             uint32 `json:"deadline"` // 截止时间（以秒为单位）
	IsPrefixalScope     int    `json:"isPrefixalScope,omitempty"`
	InsertOnly          uint16 `json:"insertOnly,omitempty"` // 若非0, 即使Scope为 Bucket:Key 的形式也是insert only
	DetectMime          uint8  `json:"detectMime,omitempty"` // 若非0, 则服务端根据内容自动确定 MimeType
	FsizeLimit          int64  `json:"fsizeLimit,omitempty"`
	MimeLimit           string `json:"mimeLimit,omitempty"`
	SaveKey             string `json:"saveKey,omitempty"`
	CallbackFetchKey    uint8  `json:"callbackFetchKey,omitempty"`
	CallbackURL         string `json:"callbackUrl,omitempty"`
	CallbackHost        string `json:"callbackHost,omitempty"`
	CallbackBody        string `json:"callbackBody,omitempty"`
	CallbackBodyType    string `json:"callbackBodyType,omitempty"`
	ReturnURL           string `json:"returnUrl,omitempty"`
	ReturnBody          string `json:"returnBody,omitempty"`
	PersistentOps       string `json:"persistentOps,omitempty"`
	PersistentNotifyURL string `json:"persistentNotifyUrl,omitempty"`
	PersistentPipeline  string `json:"persistentPipeline,omitempty"`
	EndUser             string `json:"endUser,omitempty"`
	DeleteAfterDays     int    `json:"deleteAfterDays,omitempty"`
	FileType            int    `json:"fileType,omitempty"`
}

// UploadToken 方法用来进行上传凭证的生成
func (p *PutPolicy) UploadToken(mac *qbox.Mac) (token string) {
	if p.Expires == 0 {
		p.Expires = 3600 // 1 hour
	}
	p.Expires += uint32(time.Now().Unix())

	putPolicyJSON, _ := json.Marshal(p)
	token = mac.SignWithData(putPolicyJSON)
	return
}

func getAkBucketFromUploadToken(token string) (ak, bucket string, err error) {
	items := strings.Split(token, ":")
	if len(items) != 3 {
		err = errors.New("invalid upload token, format error")
		return
	}

	ak = items[0]
	policyBytes, dErr := base64.URLEncoding.DecodeString(items[2])
	if dErr != nil {
		err = errors.New("invalid upload token, invalid put policy")
		return
	}

	putPolicy := PutPolicy{}
	uErr := json.Unmarshal(policyBytes, &putPolicy)
	if uErr != nil {
		err = errors.New("invalid upload token, invalid put policy")
		return
	}

	bucket = strings.Split(putPolicy.Scope, ":")[0]
	return
}
