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

package error

import (
	"errors"
	"fmt"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
)

func ErrHandler(err error) string {
	if err != nil && err.Error() != "" {
		vmName := config.GetAgentVmName()
		return fmt.Errorf("current task execution vm: %s, time: %s error: %v", vmName, time.Now().Format("2006-01-02 15:04:05"), err).Error()
	}
	return ""
}

var (
	ErrVerrifyAgentFailedWithNoConfig = errors.New("verify agent failed with no config")
)
