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
	"fmt"
	"strings"
	"sync"

	"github.com/koderover/zadig/pkg/tool/httpclient"
)

// 资源管理相关的默认域名
const (
	DefaultRsHost  = "rs.qiniu.com"
	DefaultRsfHost = "rsf.qiniu.com"
	DefaultAPIHost = "api.qiniu.com"
)

// Zone 为空间对应的机房属性，主要包括了上传，资源管理等操作的域名
type Zone struct {
	SrcUpHosts []string
	CdnUpHosts []string
	RsHost     string
	RsfHost    string
	APIHost    string
	IovipHost  string
}

// ZoneHuadong 表示华东机房
var ZoneHuadong = Zone{
	SrcUpHosts: []string{
		"up.qiniup.com",
		"up-nb.qiniup.com",
		"up-xs.qiniup.com",
	},
	CdnUpHosts: []string{
		"upload.qiniup.com",
		"upload-nb.qiniup.com",
		"upload-xs.qiniup.com",
	},
	RsHost:    "rs.qiniu.com",
	RsfHost:   "rsf.qiniu.com",
	APIHost:   "api.qiniu.com",
	IovipHost: "iovip.qbox.me",
}

// UcHost 为查询空间相关域名的API服务地址
const UcHost = "https://uc.qbox.me"

// UcQueryRet 为查询请求的回复
type UcQueryRet struct {
	TTL int                            `json:"ttl"`
	Io  map[string]map[string][]string `json:"io"`
	Up  map[string]UcQueryUp           `json:"up"`
}

// UcQueryUp 为查询请求回复中的上传域名信息
type UcQueryUp struct {
	Main   []string `json:"main,omitempty"`
	Backup []string `json:"backup,omitempty"`
	Info   string   `json:"info,omitempty"`
}

var (
	zoneMutext sync.RWMutex
	zoneCache  = make(map[string]*Zone)
)

// GetZone 用来根据ak和bucket来获取空间相关的机房信息
func GetZone(ak, bucket string) (zone *Zone, err error) {
	zoneID := fmt.Sprintf("%s:%s", ak, bucket)
	//check from cache
	zoneMutext.RLock()
	if v, ok := zoneCache[zoneID]; ok {
		zone = v
	}
	zoneMutext.RUnlock()
	if zone != nil {
		return
	}

	//query from server
	reqURL := fmt.Sprintf("%s/v2/query?ak=%s&bucket=%s", UcHost, ak, bucket)
	var ret UcQueryRet
	_, qErr := httpclient.Get(
		reqURL,
		httpclient.SetResult(&ret),
		httpclient.SetHeader("Content-Type", "application/x-www-form-urlencoded"),
	)
	//qErr := rpc.DefaultClient.CallWithForm(ctx, &ret, "GET", reqURL, nil)
	if qErr != nil {
		err = fmt.Errorf("query zone error, %s", qErr.Error())
		return
	}

	ioHost := ret.Io["src"]["main"][0]
	srcUpHosts := ret.Up["src"].Main
	if ret.Up["src"].Backup != nil {
		srcUpHosts = append(srcUpHosts, ret.Up["src"].Backup...)
	}
	cdnUpHosts := ret.Up["acc"].Main
	if ret.Up["acc"].Backup != nil {
		cdnUpHosts = append(cdnUpHosts, ret.Up["acc"].Backup...)
	}

	zone = &Zone{
		SrcUpHosts: srcUpHosts,
		CdnUpHosts: cdnUpHosts,
		IovipHost:  ioHost,
		RsHost:     DefaultRsHost,
		RsfHost:    DefaultRsfHost,
		APIHost:    DefaultAPIHost,
	}

	//set specific hosts if possible
	setSpecificHosts(ioHost, zone)

	zoneMutext.Lock()
	zoneCache[zoneID] = zone
	zoneMutext.Unlock()
	return
}

func setSpecificHosts(ioHost string, zone *Zone) {
	if strings.Contains(ioHost, "-z1") {
		zone.RsHost = "rs-z1.qiniu.com"
		zone.RsfHost = "rsf-z1.qiniu.com"
		zone.APIHost = "api-z1.qiniu.com"
	} else if strings.Contains(ioHost, "-z2") {
		zone.RsHost = "rs-z2.qiniu.com"
		zone.RsfHost = "rsf-z2.qiniu.com"
		zone.APIHost = "api-z2.qiniu.com"
	} else if strings.Contains(ioHost, "-na0") {
		zone.RsHost = "rs-na0.qiniu.com"
		zone.RsfHost = "rsf-na0.qiniu.com"
		zone.APIHost = "api-na0.qiniu.com"
	}
}
