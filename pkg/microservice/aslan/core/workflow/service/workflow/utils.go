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

package workflow

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/setting"
)

func validateHookNames(hookNames []string) error {
	names := sets.NewString()
	for _, name := range hookNames {
		if name == "" {
			return fmt.Errorf("empty name is not allowed")
		}
		if !setting.ValidName.MatchString(name) {
			return fmt.Errorf("invalid name. a valid name must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character")
		}
		if names.Has(name) {
			return fmt.Errorf("duplicated webhook name found: %s", name)
		}
		names.Insert(name)
	}

	return nil
}
