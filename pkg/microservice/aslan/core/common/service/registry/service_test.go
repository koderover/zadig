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
	_ "github.com/koderover/zadig/pkg/util/testing"
)

//func Test_v2RegistryService_ListRepoImages(t *testing.T) {
//	assert := require.New(t)
//	type args struct {
//		option ListRepoImagesOption
//		log    *zap.SugaredLogger
//	}
//	tests := []struct {
//		name     string
//		s        *v2RegistryService
//		args     args
//		wantResp *ReposResp
//		wantErr  bool
//	}{
//		{
//			"list tags of library",
//			&v2RegistryService{},
//			args{
//				ListRepoImagesOption{
//					Endpoint{
//						"https://n7832lxy.mirror.aliyuncs.com",
//						"",
//						"",
//						"",
//						"library",
//					},
//					[]string{"mysql", "alpine"},
//				},
//				log.SugaredLogger(),
//			},
//			&ReposResp{Total: 2},
//			false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			s := &v2RegistryService{}
//			gotResp, err := s.ListRepoImages(tt.args.option, tt.args.log)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("v2RegistryService.ListRepoImages() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//
//			assert.Nil(err)
//			assert.Equal(gotResp.Total, tt.wantResp.Total)
//			if tt.args.option.Addr == "https://n7832lxy.mirror.aliyuncs.com" {
//				for _, repo := range gotResp.Repos {
//					assert.Contains(repo.Tags, "latest")
//				}
//			}
//		})
//	}
//}
//
//func Test_v2RegistryService_GetImageInfo(t *testing.T) {
//	type args struct {
//		option GetRepoImageDetailOption
//		log    *zap.SugaredLogger
//	}
//	tests := []struct {
//		name    string
//		s       *v2RegistryService
//		args    args
//		wantDi  *models.DeliveryImage
//		wantErr bool
//	}{
//		{
//			"list tag info of mysql:5.7",
//			&v2RegistryService{},
//			args{
//				GetRepoImageDetailOption{
//					Endpoint{
//						"https://n7832lxy.mirror.aliyuncs.com",
//						"",
//						"",
//						"",
//						"library",
//					},
//					"mysql",
//					"5.7.23",
//				},
//				log.SugaredLogger(),
//			},
//			&models.DeliveryImage{ImageDigest: "sha256:953b53af26805d82eca95f28df6ae82e8e15cd1e587b4c5cd06a78be80e84050"},
//			false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			s := &v2RegistryService{}
//			gotDi, err := s.GetImageInfo(tt.args.option, tt.args.log)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("v2RegistryService.GetImageInfo() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if err == nil {
//				if gotDi.ImageDigest != tt.wantDi.ImageDigest {
//					t.Errorf("v2RegistryService.GetImageInfo() = %v, want %v", gotDi, tt.wantDi)
//				}
//			}
//		})
//	}
//}
//
//func TestSwrListRepoImage(t *testing.T) {
//	s := &SwrService{}
//	listRepoImagesOption := ListRepoImagesOption{
//		Endpoint: Endpoint{
//			Namespace: "lilian",
//			Ak:        "",
//			Sk:        "",
//		},
//		Repos: []string{"nginx-test"},
//	}
//	_, err := s.ListRepoImages(listRepoImagesOption, log.SugaredLogger())
//	assert.Nil(t, err)
//}
//
//func TestSwrImageInfo(t *testing.T) {
//	s := &SwrService{}
//	getRepoImageDetailOption := GetRepoImageDetailOption{
//		Endpoint: Endpoint{
//			Region:    "cn-north-4",
//			Namespace: "lilian",
//			Ak:        "",
//			Sk:        "",
//		},
//		Tag:   "20210712210942-34-master",
//		Image: "nginx-test",
//	}
//	_, err := s.GetImageInfo(getRepoImageDetailOption, log.SugaredLogger())
//	assert.Nil(t, err)
//}
