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
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/config"
	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	templaterepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb/template"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/base"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type basicImageRef struct {
	productName string
	resource    string
	name        string
}

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
			Value:      "sonarsource/sonar-scanner-cli:5.0.1",
			Label:      "sonar:5.0.1",
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
	}
	imageList, err := commonrepo.NewBasicImageColl().List(opt)
	if err != nil {
		log.Errorf("BasicImage.List error: %v", err)
		return nil, e.ErrListBasicImages
	}

	resp := make([]*commonmodels.BasicImage, 0)
	for _, image := range imageList {
		if imageType != "sonar" {
			if image.ImageType == "sonar" {
				continue
			}
		}
		resp = append(resp, image)
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
	// Exclude the current image so retaining its existing value remains valid.
	for _, image := range resp {
		if image.ID.Hex() != id {
			log.Errorf("update BasicImage failed, value:%s has existed", args.Value)
			return e.ErrUpdateBasicImage.AddDesc(fmt.Sprintf("value:%s has existed", args.Value))
		}
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
	refs := make([]basicImageRef, 0)
	seen := make(map[string]struct{})
	var listErr error

	builds, err := commonrepo.NewBuildColl().List(&commonrepo.BuildListOption{BasicImageID: id})
	if err != nil {
		log.Errorf("list builds by basic image %s error: %v", id, err)
		listErr = err
	}
	for _, build := range builds {
		if build == nil {
			continue
		}
		refs = appendBasicImageRef(refs, seen, basicImageRef{
			productName: build.ProductName,
			resource:    "构建",
			name:        build.Name,
		})
	}

	tests, err := commonrepo.NewTestingColl().List(&commonrepo.ListTestOption{BasicImageID: id})
	if err != nil {
		log.Errorf("list testings by basic image %s error: %v", id, err)
		if listErr == nil {
			listErr = err
		}
	}
	for _, test := range tests {
		if test == nil {
			continue
		}
		refs = appendBasicImageRef(refs, seen, basicImageRef{
			productName: test.ProductName,
			resource:    "测试",
			name:        test.Name,
		})
	}

	pipelines, err := commonrepo.NewPipelineColl().List(&commonrepo.PipelineListOption{})
	if err != nil {
		log.Errorf("list pipelines for basic image %s error: %v", id, err)
		if listErr == nil {
			listErr = err
		}
	}
	for _, pipeline := range pipelines {
		if pipeline == nil {
			continue
		}
		for _, subTask := range pipeline.SubTasks {
			pre, err := base.ToPreview(subTask)
			if err != nil {
				continue
			}
			switch pre.TaskType {
			case config.TaskBuild:
				// 校验是否被服务工作流的构建使用
				task, err := base.ToBuildTask(subTask)
				if err != nil || task == nil || task.ImageID != id {
					continue
				}
				name := pipeline.Name
				if task.ServiceName != "" {
					name = fmt.Sprintf("%s（构建 %s）", pipeline.Name, task.ServiceName)
				}
				refs = appendBasicImageRef(refs, seen, basicImageRef{
					productName: pipeline.ProductName,
					resource:    "工作流",
					name:        name,
				})
			case config.TaskTestingV2:
				// 校验是否被服务工作流的测试使用
				testing, err := base.ToTestingTask(subTask)
				if err != nil || testing == nil || testing.ImageID != id {
					continue
				}
				name := pipeline.Name
				if testing.TestName != "" {
					name = fmt.Sprintf("%s（测试 %s）", pipeline.Name, testing.TestName)
				}
				refs = appendBasicImageRef(refs, seen, basicImageRef{
					productName: pipeline.ProductName,
					resource:    "工作流",
					name:        name,
				})
			}
		}
	}

	if len(refs) != 0 {
		return basicImageInUseError(id, refs, listErr != nil, log)
	}
	if listErr != nil {
		return e.NewWithDesc(e.ErrDeleteBasicImage, "检查基础镜像引用失败，请稍后重试")
	}

	err = commonrepo.NewBasicImageColl().Delete(id)
	if err != nil {
		log.Errorf("BasicImage.Delete %s error: %v", id, err)
		return e.ErrDeleteBasicImage
	}
	return nil
}

func appendBasicImageRef(refs []basicImageRef, seen map[string]struct{}, ref basicImageRef) []basicImageRef {
	key := ref.resource + "\x00" + ref.productName + "\x00" + ref.name
	if _, ok := seen[key]; ok {
		return refs
	}
	seen[key] = struct{}{}
	return append(refs, ref)
}

func basicImageInUseError(id string, refs []basicImageRef, incomplete bool, log *zap.SugaredLogger) error {
	desc := describeBasicImageRefs(refs, projectDisplayNames(refs, log))
	if incomplete {
		desc += "，部分引用检查失败，列表可能不完整"
	}
	log.Errorf("BasicImage has been used, image id:%s, detail:%s", id, desc)
	return e.NewWithDesc(e.ErrDeleteUsedBasicImage, desc)
}

func describeBasicImageRefs(refs []basicImageRef, projectNames map[string]string) string {
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, formatBasicImageRef(ref, projectNames))
	}
	return "引用位置：" + strings.Join(parts, "，")
}

func formatBasicImageRef(ref basicImageRef, projectNames map[string]string) string {
	target := fmt.Sprintf("%s %s", ref.resource, ref.name)
	if strings.TrimSpace(ref.productName) == "" {
		return target + "（未关联项目）"
	}
	return fmt.Sprintf("项目 %s，%s", formatProjectLabel(ref.productName, projectNames), target)
}

func formatProjectLabel(productName string, projectNames map[string]string) string {
	alias := strings.TrimSpace(projectNames[productName])
	if alias == "" || alias == productName {
		return productName
	}
	return fmt.Sprintf("%s（%s）", alias, productName)
}

func projectDisplayNames(refs []basicImageRef, log *zap.SugaredLogger) map[string]string {
	names := make(map[string]string)
	keys := make([]string, 0)
	seen := make(map[string]struct{})
	for _, ref := range refs {
		productName := strings.TrimSpace(ref.productName)
		if productName == "" {
			continue
		}
		if _, ok := seen[productName]; ok {
			continue
		}
		seen[productName] = struct{}{}
		keys = append(keys, productName)
	}
	if len(keys) == 0 {
		return names
	}

	projects, err := templaterepo.NewProductColl().ListProjectBriefs(keys)
	if err != nil {
		log.Warnf("list project names for basic image refs failed: %v", err)
		return names
	}
	for _, project := range projects {
		if project == nil {
			continue
		}
		names[project.Name] = project.Alias
	}
	return names
}
