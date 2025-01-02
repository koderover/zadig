/*
 * Copyright 2024 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package migrate

import (
	"github.com/koderover/zadig/v2/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/v2/pkg/shared/handler"
)

func init() {
	upgradepath.RegisterHandler("3.2.0", "3.2.1", V320ToV321)
	upgradepath.RegisterHandler("3.2.1", "3.2.0", V321ToV320)
}

func V320ToV321() error {
	ctx := handler.NewBackgroupContext()
	ctx.Logger.Infof("-------- start 321 ua --------")

	return nil
}

func V321ToV320() error {
	return nil
}
