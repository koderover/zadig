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

package registry

import (
	"reflect"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/koderover/zadig/lib/microservice/aslan/core/common/dao/models"
	"github.com/koderover/zadig/lib/tool/xlog"
)

func Test_v2RegistryService_ListRepoImages(t *testing.T) {
	assert := require.New(t)
	type args struct {
		option ListRepoImagesOption
		log    *xlog.Logger
	}
	tests := []struct {
		name     string
		s        *v2RegistryService
		args     args
		wantResp *ReposResp
		wantErr  bool
	}{
		{
			"list tags of library",
			&v2RegistryService{},
			args{
				ListRepoImagesOption{
					Endpoint{
						"https://n7832lxy.mirror.aliyuncs.com",
						"",
						"",
						"library",
					},
					[]string{"mysql", "alpine"},
				},
				xlog.NewDummy(),
			},
			&ReposResp{Total: 2},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &v2RegistryService{}
			gotResp, err := s.ListRepoImages(tt.args.option, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("v2RegistryService.ListRepoImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Nil(err)
			assert.Equal(gotResp.Total, tt.wantResp.Total)
			if tt.args.option.Addr == "https://n7832lxy.mirror.aliyuncs.com" {
				for _, repo := range gotResp.Repos {
					assert.Contains(repo.Tags, "latest")
				}
			}
		})
	}
}

func Test_v2RegistryService_GetImageInfo(t *testing.T) {
	type args struct {
		option GetRepoImageDetailOption
		log    *xlog.Logger
	}
	tests := []struct {
		name    string
		s       *v2RegistryService
		args    args
		wantDi  *models.DeliveryImage
		wantErr bool
	}{
		{
			"list tag info of mysql:5.7",
			&v2RegistryService{},
			args{
				GetRepoImageDetailOption{
					Endpoint{
						"https://n7832lxy.mirror.aliyuncs.com",
						"",
						"",
						"library",
					},
					"mysql",
					"5.7.23",
				},
				xlog.NewDummy(),
			},
			&models.DeliveryImage{ImageDigest: "sha256:953b53af26805d82eca95f28df6ae82e8e15cd1e587b4c5cd06a78be80e84050"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &v2RegistryService{}
			gotDi, err := s.GetImageInfo(tt.args.option, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("v2RegistryService.GetImageInfo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if gotDi.ImageDigest != tt.wantDi.ImageDigest {
					t.Errorf("v2RegistryService.GetImageInfo() = %v, want %v", gotDi, tt.wantDi)
				}
			}
		})
	}
}

func TestReverseStringSlice_Len(t *testing.T) {
	a := []string{"1", "3", "2"}
	sort.Sort(ReverseStringSlice(a))
	if !reflect.DeepEqual(a, []string{"2", "3", "1"}) {
		t.Error("reverse sort not works")
	}
}
