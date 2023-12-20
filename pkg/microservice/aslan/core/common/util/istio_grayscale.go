/*
Copyright 2023 The KodeRover Authors.

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

package util

import (
	"context"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	zadigutil "github.com/koderover/zadig/v2/pkg/util"
)

func EnsureIstioGrayConfig(ctx context.Context, baseEnv *commonmodels.Product) error {
	if baseEnv.IstioGrayscale.Enable && baseEnv.IstioGrayscale.IsBase {
		return nil
	}

	baseEnv.IstioGrayscale = commonmodels.IstioGrayscale{
		Enable: true,
		IsBase: true,
	}

	return commonrepo.NewProductColl().Update(baseEnv)
}

func FetchGrayEnvs(ctx context.Context, productName, clusterID, baseEnvName string) ([]*commonmodels.Product, error) {
	return commonrepo.NewProductColl().List(&commonrepo.ProductListOptions{
		Name:                  productName,
		ClusterID:             clusterID,
		IstioGrayscaleEnable:  zadigutil.GetBoolPointer(true),
		IstioGrayscaleIsBase:  zadigutil.GetBoolPointer(false),
		IstioGrayscaleBaseEnv: zadigutil.GetStrPointer(baseEnvName),
		Production:            zadigutil.GetBoolPointer(true),
	})
}
