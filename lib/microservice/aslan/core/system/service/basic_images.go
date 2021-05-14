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

package service

import (
	"time"

	commonmodels "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	commonrepo "github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/repo"
	e "github.com/koderover/zadig/lib/tool/errors"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func InitbasicImageInfos() []*commonmodels.BasicImage {
	basicImageInfos := []*commonmodels.BasicImage{
		{
			Value:      "xenial",
			Label:      "ubuntu 16.04",
			ImageFrom:  "koderover",
			CreateTime: time.Now().Unix(),
			UpdateTime: time.Now().Unix(),
			UpdateBy:   "system",
		},
		{
			Value:      "bionic",
			Label:      "ubuntu 18.04",
			ImageFrom:  "koderover",
			CreateTime: time.Now().Unix(),
			UpdateTime: time.Now().Unix(),
			UpdateBy:   "system",
		},
		{
			Value:      "focal",
			Label:      "ubuntu 20.04",
			ImageFrom:  "koderover",
			CreateTime: time.Now().Unix(),
			UpdateTime: time.Now().Unix(),
			UpdateBy:   "system",
		},
	}

	return basicImageInfos
}

func GetBasicImage(id string, log *xlog.Logger) (*commonmodels.BasicImage, error) {
	resp, err := commonrepo.NewBasicImageColl().Find(id)
	if err != nil {
		log.Errorf("BasicImage.Find %s error: %v", id, err)
		return resp, e.ErrGetBasicImage
	}
	return resp, nil
}

func ListBasicImages(imageFrom string, log *xlog.Logger) ([]*commonmodels.BasicImage, error) {
	opt := &commonrepo.BasicImageOpt{
		ImageFrom: imageFrom,
	}
	resp, err := commonrepo.NewBasicImageColl().List(opt)
	if err != nil {
		log.Errorf("BasicImage.List error: %v", err)
		return resp, e.ErrListBasicImages
	}
	return resp, nil
}
