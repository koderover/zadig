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
	"os"
	"path/filepath"

	utilfs "github.com/koderover/zadig/v2/pkg/util/fs"
)

// GoCacheManager is deprecated
type GoCacheManager struct{}

func NewGoCacheManager() *GoCacheManager {
	return &GoCacheManager{}
}

func (gcm *GoCacheManager) Archive(source, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(dest), ".zadig-cache-*.tar.gz")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if err = temp.Close(); err != nil {
		return err
	}
	defer os.Remove(tempName)

	if err = utilfs.Tar(os.DirFS(source), tempName); err != nil {
		return err
	}
	return os.Rename(tempName, dest)
}
