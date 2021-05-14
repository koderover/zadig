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

package client

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/koderover/zadig/lib/setting"
	"github.com/koderover/zadig/lib/tool/xlog"
)

const (
	testApiHost  = "os.koderover.com"
	testApiToken = "token"
)

func TestGenListWorkFlowReq(t *testing.T) {
	client := NewAslanClient(
		testApiHost,
		testApiToken,
	)
	req, _ := client.genListWorkFlowReq(xlog.NewDummy())
	assert.Equal(t, fmt.Sprintf("%s/workflow/workflow", testApiHost), req.URL.Path)
	assert.Equal(t, fmt.Sprintf("%s %s", setting.ROOTAPIKEY, testApiToken), req.Header.Get(authHeaderKey))
}
