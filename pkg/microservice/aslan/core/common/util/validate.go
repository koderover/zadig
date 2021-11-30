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
	"strconv"
	"strings"

	"github.com/koderover/zadig/pkg/setting"
)

func CheckDefineResourceParam(req setting.Request, reqSpec setting.RequestSpec) error {
	if req != setting.DefineRequest {
		return nil
	}
	cpuLimit := strings.TrimSuffix(reqSpec.CpuLimit, setting.CpuUintM)
	cpuLimitInt, err := strconv.Atoi(cpuLimit)
	if err != nil || cpuLimitInt <= 1 {
		return fmt.Errorf("Parameter res_req_spc.cpu_limit must be greater than 1m")
	}
	memoryLimit := strings.TrimSuffix(reqSpec.MemoryLimit, setting.MemoryUintMi)
	memoryLimitInt, err := strconv.Atoi(memoryLimit)
	if err != nil || memoryLimitInt <= 1 {
		return fmt.Errorf("Parameter res_req_spc.memory_limit must be greater than 1Mi")
	}
	return nil
}
