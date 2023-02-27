/*
 * Copyright 2023 The KodeRover Authors.
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

package apollo

import (
	"fmt"
	"testing"
)

func TestName(t *testing.T) {
	//cli := NewClient("http://101.43.78.136:8070", "20339d6ed820fb89aac1819d3deff0ee2e857cb9")
	cli := NewClient("http://127.0.0.1:8081", "20339d6ed820fb89aac1819d3deff0ee2e857cb9")
	resp, err := cli.ListApp()
	//resp, err := cli.GetAppEnvsAndClusters("SampleApp")
	fmt.Println(err)
	panic(resp)
	panic(err)
}
