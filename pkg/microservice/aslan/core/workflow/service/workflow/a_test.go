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

package workflow

import (
	"context"
	"testing"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	mongotool "github.com/koderover/zadig/pkg/tool/mongo"
)

var raw = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  selector:
    matchLabels:
      app: nginx
      zadigx-release-version: original
  replicas: 1 # 告知 Deployment 运行 2 个与该模板匹配的 Pod
  template:
    metadata:
      labels:
        app: nginx
        zadigx-release-version: original
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
      - name: nginx2
        image: nginx:1.14.2
        ports:
        - containerPort: 81
`

func TestName(t *testing.T) {
	log.Init(&log.Config{
		Level:    "debug",
		NoCaller: true,
	})
	mongotool.Init(context.TODO(), "mongodb://localhost:27017")
	viper.Set(setting.ENVAslanDBName, "zadig_ee_test")

	//str, err := GetMseServiceYaml("cosmos-1", "dev", "n-4", "v1.2")
	//if err != nil {
	//	t.Errorf("err:%v", err)
	//	return
	//}
	//t.Logf("%s", str)

	str, err := RenderMseServiceYaml("v1.4", &models.MseGrayReleaseService{
		ServiceName: "n-3",
		Replicas:    4,
		YamlContent: raw,
		ServiceAndImage: []*models.MseGrayReleaseServiceModuleAndImage{
			{
				ServiceModule: "nginx",
				Image:         "123",
			},
			{
				ServiceModule: "nginx2",
				Image:         "456",
			},
		},
	})
	if err != nil {
		t.Errorf("err:%v", err)
		return
	}
	t.Logf("%s", str)
}
