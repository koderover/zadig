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

package util

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/shared/client/plutusvendor"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func checkGpuResourceParam(gpuLimit string) error {
	gpuLimit = strings.ReplaceAll(gpuLimit, " ", "")
	requestPair := strings.Split(gpuLimit, ":")
	if len(requestPair) != 2 {
		return fmt.Errorf("format of gpu request is incorrect")
	}
	_, err := resource.ParseQuantity(requestPair[1])
	if err != nil {
		return fmt.Errorf("failed to parse resource quantity, err: %s", err)
	}
	return err
}

func CheckDefineResourceParam(req setting.Request, reqSpec setting.RequestSpec) error {
	if req != setting.DefineRequest {
		return nil
	}
	if len(reqSpec.GpuLimit) > 0 {
		return checkGpuResourceParam(reqSpec.GpuLimit)
	}
	//if reqSpec.CpuLimit < 1 {
	//	return fmt.Errorf("Parameter res_req_spc.cpu_limit must be greater than 1m")
	//}
	//if reqSpec.MemoryLimit < 1 {
	//	return fmt.Errorf("Parameter res_req_spc.memory_limit must be greater than 1Mi")
	//}
	return nil
}

func CheckZadigProfessionalLicense() error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !ValidateZadigProfessionalLicense(licenseStatus) {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	return nil
}

func CheckZadigEnterpriseLicense() error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !ValidateZadigEnterpriseLicense(licenseStatus) {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	return nil
}

func ValidateZadigProfessionalLicense(licenseStatus *plutusvendor.ZadigXLicenseStatus) bool {
	if !((licenseStatus.Type == plutusvendor.ZadigSystemTypeProfessional ||
		licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise) &&
		licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		return false
	}
	return true
}

func ValidateZadigEnterpriseLicense(licenseStatus *plutusvendor.ZadigXLicenseStatus) bool {
	if !(licenseStatus.Type == plutusvendor.ZadigSystemTypeEnterprise &&
		licenseStatus.Status == plutusvendor.ZadigXLicenseStatusNormal) {
		return false
	}
	return true
}

func CheckZadigLicenseFeatureSae() error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !ValidateZadigProfessionalLicense(licenseStatus) {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	for _, feature := range licenseStatus.Features {
		if feature == plutusvendor.ZadigLicenseFeatureSae {
			return nil
		}
	}
	return e.ErrLicenseInvalid.AddDesc("")
}

func CheckZadigLicenseFeatureDelivery() error {
	licenseStatus, err := plutusvendor.New().CheckZadigXLicenseStatus()
	if err != nil {
		return fmt.Errorf("failed to validate zadig license status, error: %s", err)
	}
	if !ValidateZadigProfessionalLicense(licenseStatus) {
		return e.ErrLicenseInvalid.AddDesc("")
	}
	for _, feature := range licenseStatus.Features {
		if feature == plutusvendor.ZadigLicenseFeatureDelivery {
			return nil
		}
	}
	return e.ErrLicenseInvalid.AddDesc("")
}
