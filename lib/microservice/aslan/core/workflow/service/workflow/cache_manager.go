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
	"io/ioutil"
	"path/filepath"

	"gopkg.in/mholt/archiver.v3"
)

// GoCacheManager is deprecated
type GoCacheManager struct{}

func NewGoCacheManager() *GoCacheManager {
	return &GoCacheManager{}
}

func (gcm *GoCacheManager) Archive(source, dest string) error {
	var sources []string
	if files, err := ioutil.ReadDir(source); err != nil {
		return err
	} else {
		for _, f := range files {
			sources = append(sources, filepath.Join(source, f.Name()))
		}
	}

	return getAchiver().Archive(sources, dest)
}

func getAchiver() *archiver.TarGz {
	return &archiver.TarGz{
		Tar: &archiver.Tar{
			OverwriteExisting:      true,
			MkdirAll:               true,
			ImplicitTopLevelFolder: false,
		},
	}
}
