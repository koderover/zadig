package sae

import (
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sae "github.com/alibabacloud-go/sae-20190506/client"
	"github.com/alibabacloud-go/tea/tea"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func NewClient(saeModel *models.SAE, regionID string) (result *sae.Client, err error) {
	config := &openapi.Config{
		AccessKeyId:     tea.String(saeModel.AccessKeyId),
		AccessKeySecret: tea.String(saeModel.AccessKeySecret),
	}

	config.Endpoint = tea.String(fmt.Sprintf("sae.%s.aliyuncs.com", regionID))
	result, err = sae.NewClient(config)
	return result, err
}
