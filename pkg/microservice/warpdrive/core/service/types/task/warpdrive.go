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

package task

import (
	"github.com/koderover/zadig/pkg/types"
)

type PipelineCtx struct {
	DockerHost        string
	Workspace         string
	DistDir           string
	DockerMountDir    string
	ConfigMapMountDir string
	MultiRun          bool

	// New since V1.10.0.
	Cache        types.Cache
	CacheEnable  bool
	CacheDirType types.CacheDirType
	CacheUserDir string

	UseHostDockerDaemon bool
}

type JobCtxBuilder struct {
	JobName       string
	ArchiveBucket string
	ArchiveFile   string
	PipelineCtx   *PipelineCtx
	JobCtx        JobCtx
	Installs      []*Install
}
