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

package kodo

// Config 为文件上传，资源管理等配置
type Config struct {
	Zone          *Zone //空间所在的机房
	UseHTTPS      bool  //是否使用https域名
	UseCdnDomains bool  //是否使用cdn加速域名
}
