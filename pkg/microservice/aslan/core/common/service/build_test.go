/*
Copyright 2026 The KodeRover Authors.

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

package service

import (
	"reflect"
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	"github.com/koderover/zadig/v2/pkg/util"
)

func TestGetBuildDoesNotMutateCaches(t *testing.T) {
	for _, templateID := range []string{"", "template"} {
		t.Run("template="+templateID, func(t *testing.T) {
			cached := &commonmodels.Build{
				TemplateID: templateID,
				PreBuild: &commonmodels.PreBuild{Envs: commonmodels.KeyValList{
					{Key: "ENV", Value: "default", ChoiceOption: []string{"default", "A"}},
				}},
				Targets: []*commonmodels.ServiceModuleTarget{
					{
						ServiceWithModule: commonmodels.ServiceWithModule{ServiceName: "A", ServiceModule: "a"},
						Envs:              commonmodels.KeyValList{{Key: "ENV", Value: "A"}},
					},
					{ServiceWithModule: commonmodels.ServiceWithModule{ServiceName: "B", ServiceModule: "b"}},
				},
			}
			template := &commonmodels.BuildTemplate{PreBuild: cached.PreBuild}
			svc := NewBuildService()
			svc.BuildMap.Store("build", cached)
			svc.BuildTemplateMap.Store(templateID, template)
			originalBuild := new(commonmodels.Build)
			originalTemplate := new(commonmodels.BuildTemplate)
			if err := util.DeepCopy(originalBuild, cached); err != nil {
				t.Fatal(err)
			}
			if err := util.DeepCopy(originalTemplate, template); err != nil {
				t.Fatal(err)
			}

			a, err := svc.GetBuild("build", "A", "a")
			if err != nil {
				t.Fatal(err)
			}
			wantA := "default"
			if templateID != "" {
				wantA = "A"
			}
			if got := a.PreBuild.Envs[0].Value; got != wantA {
				t.Fatalf("A ENV = %q, want %q", got, wantA)
			}
			b, err := svc.GetBuild("build", "B", "b")
			if err != nil {
				t.Fatal(err)
			}
			if got := b.PreBuild.Envs[0].Value; got != "default" {
				t.Errorf("B inherited A's ENV: got %q, want default", got)
			}
			if got := a.PreBuild.Envs[0].Value; got != wantA {
				t.Errorf("reading B changed A's ENV: got %q, want %q", got, wantA)
			}
			a.PreBuild.Envs[0].Value = "caller mutation"
			a.PreBuild.Envs[0].ChoiceOption[0] = "caller mutation"
			if got := b.PreBuild.Envs[0].Value; got != "default" {
				t.Errorf("mutating A changed B's ENV: %q", got)
			}
			if !reflect.DeepEqual(cached, originalBuild) {
				t.Error("cached build was mutated")
			}
			if !reflect.DeepEqual(template, originalTemplate) {
				t.Error("cached template was mutated")
			}
		})
	}
}
