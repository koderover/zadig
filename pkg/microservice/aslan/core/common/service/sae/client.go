package sae

import (
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sae20190506 "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func NewClient(sae *models.SAE, regionID string) (result *sae20190506.Client, err error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(sae.AccessKeyId),
		AccessKeySecret: tea.String(sae.AccessKeySecret),
	}

	config.Endpoint = tea.String(fmt.Sprintf("sae.%s.aliyuncs.com", regionID))
	result, err = sae20190506.NewClient(config)
	return result, err
}
