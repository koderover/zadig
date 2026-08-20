/*
Copyright 2026 The KodeRover Authors.

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
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

type OpenAPICustomImage struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Value      string `json:"value"`
	ImageType  string `json:"image_type"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
	UpdateBy   string `json:"update_by"`
}

type OpenAPICustomImageListResp struct {
	CustomImages []*OpenAPICustomImage `json:"custom_images"`
}

type OpenAPICustomImageReq struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	ImageType string `json:"image_type"`
}

func (req OpenAPICustomImageReq) Validate() error {
	if strings.TrimSpace(req.Label) == "" {
		return fmt.Errorf("label cannot be empty")
	}
	if strings.TrimSpace(req.Value) == "" {
		return fmt.Errorf("value cannot be empty")
	}
	if req.ImageType != "" && req.ImageType != "sonar" {
		return fmt.Errorf("image_type must be empty or sonar")
	}
	return nil
}

func ListCustomImagesOpenAPI(logger *zap.SugaredLogger) (*OpenAPICustomImageListResp, error) {
	images, err := commonrepo.NewBasicImageColl().List(&commonrepo.BasicImageOpt{ImageFrom: commonmodels.ImageFromCustom})
	if err != nil {
		logger.Errorf("OpenAPI: failed to list custom images, error: %s", err)
		return nil, e.ErrListBasicImages.AddErr(err)
	}

	resp := &OpenAPICustomImageListResp{CustomImages: make([]*OpenAPICustomImage, 0, len(images))}
	for _, image := range images {
		resp.CustomImages = append(resp.CustomImages, convertCustomImage(image))
	}
	return resp, nil
}

func GetCustomImageOpenAPI(id string, logger *zap.SugaredLogger) (*OpenAPICustomImage, error) {
	image, err := findCustomImage(id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, e.ErrNotFound.AddErr(err)
		}
		logger.Errorf("OpenAPI: failed to get custom image %s, error: %s", id, err)
		return nil, e.ErrGetBasicImage.AddErr(err)
	}
	return convertCustomImage(image), nil
}

func CreateCustomImageOpenAPI(req *OpenAPICustomImageReq, userName string, logger *zap.SugaredLogger) error {
	if err := req.Validate(); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}

	value := strings.TrimSpace(req.Value)
	if err := ensureCustomImageValueAvailable(value, ""); err != nil {
		return e.ErrCreateBasicImage.AddErr(err)
	}

	image := &commonmodels.BasicImage{
		Label:     strings.TrimSpace(req.Label),
		Value:     value,
		ImageFrom: commonmodels.ImageFromCustom,
		ImageType: req.ImageType,
		UpdateBy:  userName,
	}
	if err := commonrepo.NewBasicImageColl().Create(image); err != nil {
		logger.Errorf("OpenAPI: failed to create custom image, error: %s", err)
		return e.ErrCreateBasicImage.AddErr(err)
	}
	return nil
}

func UpdateCustomImageOpenAPI(id string, req *OpenAPICustomImageReq, userName string, logger *zap.SugaredLogger) error {
	if err := req.Validate(); err != nil {
		return e.ErrInvalidParam.AddErr(err)
	}

	image, err := findCustomImage(id)
	if err != nil {
		return e.ErrUpdateBasicImage.AddErr(err)
	}
	value := strings.TrimSpace(req.Value)
	if err := ensureCustomImageValueAvailable(value, image.ID.Hex()); err != nil {
		return e.ErrUpdateBasicImage.AddErr(err)
	}

	image.Label = strings.TrimSpace(req.Label)
	image.Value = value
	image.ImageType = req.ImageType
	image.UpdateBy = userName
	if err := commonrepo.NewBasicImageColl().Update(id, image); err != nil {
		logger.Errorf("OpenAPI: failed to update custom image %s, error: %s", id, err)
		return e.ErrUpdateBasicImage.AddErr(err)
	}
	return nil
}

func DeleteCustomImageOpenAPI(id string, logger *zap.SugaredLogger) error {
	if _, err := findCustomImage(id); err != nil {
		return e.ErrDeleteBasicImage.AddErr(err)
	}
	return DeleteBasicImage(id, logger)
}

func findCustomImage(id string) (*commonmodels.BasicImage, error) {
	image, err := commonrepo.NewBasicImageColl().Find(id)
	if err != nil {
		return nil, err
	}
	if image.ImageFrom != commonmodels.ImageFromCustom {
		return nil, fmt.Errorf("image %s is not a custom image", id)
	}
	return image, nil
}

func ensureCustomImageValueAvailable(value, currentID string) error {
	images, err := commonrepo.NewBasicImageColl().List(&commonrepo.BasicImageOpt{Value: value})
	if err != nil {
		return err
	}
	for _, image := range images {
		if image.ID.Hex() != currentID {
			return fmt.Errorf("value:%s has existed", value)
		}
	}
	return nil
}

func convertCustomImage(image *commonmodels.BasicImage) *OpenAPICustomImage {
	return &OpenAPICustomImage{
		ID:         image.ID.Hex(),
		Label:      image.Label,
		Value:      image.Value,
		ImageType:  image.ImageType,
		CreateTime: image.CreateTime,
		UpdateTime: image.UpdateTime,
		UpdateBy:   image.UpdateBy,
	}
}
