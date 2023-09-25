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

package migrate

import (
	"github.com/koderover/zadig/pkg/cli/upgradeassistant/internal/upgradepath"
	"github.com/koderover/zadig/pkg/tool/log"
)

func init() {
	upgradepath.RegisterHandler("1.4.0", "1.5.0", V140ToV150)
	upgradepath.RegisterHandler("1.5.0", "1.4.0", V150ToV140)
}

// V140ToV150 fill image path data for old data in product.services.containers
// use preset rules as patterns: {"image": "repository", "tag": "tag"}, {"image": "image"}
func V140ToV150() error {
	return nil
}

func V150ToV140() error {
	log.Info("Rollback data from 1.5.0 to 1.4.0")
	return nil
}
