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
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/service/base"
	e "github.com/koderover/zadig/pkg/tool/errors"
)

func InitbasicImageInfos() []*commonmodels.BasicImage {
	basicImageInfos := []*commonmodels.BasicImage{
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
		{
			Value:      "sonarsource/sonar-scanner-cli",
			Label:      "sonar:latest",
			CreateTime: time.Now().Unix(),
			UpdateTime: time.Now().Unix(),
			UpdateBy:   "system",
			ImageType:  "sonar",
		},
	}

	return basicImageInfos
}

func GetBasicImage(id string, log *zap.SugaredLogger) (*commonmodels.BasicImage, error) {
	resp, err := commonrepo.NewBasicImageColl().Find(id)
	if err != nil {
		log.Errorf("BasicImage.Find %s error: %v", id, err)
		return resp, e.ErrGetBasicImage
	}
	return resp, nil
}

func ListBasicImages(imageFrom string, imageType string, log *zap.SugaredLogger) ([]*commonmodels.BasicImage, error) {
	opt := &commonrepo.BasicImageOpt{
		ImageFrom: imageFrom,
		ImageType: imageType,
	}
	resp, err := commonrepo.NewBasicImageColl().List(opt)
	if err != nil {
		log.Errorf("BasicImage.List error: %v", err)
		return resp, e.ErrListBasicImages
	}
	return resp, nil
}

func CreateBasicImage(args *commonmodels.BasicImage, log *zap.SugaredLogger) error {
	// value已存在则不能创建
	opt := &commonrepo.BasicImageOpt{
		Value: args.Value,
	}
	resp, err := commonrepo.NewBasicImageColl().List(opt)
	if err == nil && len(resp) != 0 {
		log.Errorf("create BasicImage failed, value:%s has existed", args.Value)
		return e.ErrCreateBasicImage.AddDesc(fmt.Sprintf("value:%s has existed", args.Value))
	}

	err = commonrepo.NewBasicImageColl().Create(args)
	if err != nil {
		log.Errorf("BasicImage.Create error: %v", err)
		return e.ErrCreateBasicImage
	}
	return nil
}

func UpdateBasicImage(id string, args *commonmodels.BasicImage, log *zap.SugaredLogger) error {
	// 新的value已存在则不能更新
	opt := &commonrepo.BasicImageOpt{
		Value: args.Value,
	}
	resp, _ := commonrepo.NewBasicImageColl().List(opt)
	if len(resp) != 0 {
		log.Errorf("update BasicImage failed, value:%s has existed", args.Value)
		return e.ErrUpdateBasicImage.AddDesc(fmt.Sprintf("value:%s has existed", args.Value))
	}

	err := commonrepo.NewBasicImageColl().Update(id, args)
	if err != nil {
		log.Errorf("BasicImage.Update %s error: %v", id, err)
		return e.ErrUpdateBasicImage
	}
	return nil
}

func DeleteBasicImage(id string, log *zap.SugaredLogger) error {
	// 检查该镜像是否被引用
	buildOpt := &commonrepo.BuildListOption{BasicImageID: id}
	builds, err := commonrepo.NewBuildColl().List(buildOpt)
	if err == nil && len(builds) != 0 {
		log.Errorf("BasicImage has been used by build, image id:%s, product name:%s, build name:%s", id, builds[0].ProductName, builds[0].Name)
		return e.ErrDeleteUsedBasicImage
	}

	testOpt := &commonrepo.ListTestOption{BasicImageID: id}
	tests, err := commonrepo.NewTestingColl().List(testOpt)
	if err == nil && len(tests) != 0 {
		log.Errorf("BasicImage has been used by testing, image id:%s, product name:%s, testing name:%s", id, tests[0].ProductName, tests[0].Name)
		return e.ErrDeleteUsedBasicImage
	}

	pipelines, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err == nil {
		for _, pipeline := range pipelines {
			for _, subTask := range pipeline.SubTasks {
				pre, err := base.ToPreview(subTask)
				if err != nil {
					continue
				}
				switch pre.TaskType {
				case config.TaskBuild:
					// 校验是否被服务工作流的构建使用
					task, err := base.ToBuildTask(subTask)
					if err != nil || task == nil {
						continue
					}
					if task.ImageID == id {
						log.Errorf("BasicImage has been used by pipeline build, image id:%s, pipeline name:%s", id, pipeline.Name)
						return e.ErrDeleteUsedBasicImage
					}
				case config.TaskTestingV2:
					// 校验是否被服务工作流的测试使用
					testing, err := base.ToTestingTask(subTask)
					if err != nil || testing == nil {
						continue
					}
					if testing.ImageID == id {
						log.Errorf("BasicImage has been used by pipeline testing, image id:%s, pipeline name:%s", id, pipeline.Name)
						return e.ErrDeleteUsedBasicImage
					}
				}
			}
		}
	}

	err = commonrepo.NewBasicImageColl().Delete(id)
	if err != nil {
		log.Errorf("BasicImage.Delete %s error: %v", id, err)
		return e.ErrDeleteBasicImage
	}
	return nil
}
