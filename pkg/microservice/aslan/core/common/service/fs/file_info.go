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

package fs

import (
	"io/fs"
)

type FileInfo struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"is_dir"`
}

func GetFileInfos(fileTree fs.FS) ([]*FileInfo, error) {
	var res []*FileInfo
	err := fs.WalkDir(fileTree, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		info, _ := entry.Info()
		if info != nil {
			res = append(res, &FileInfo{
				Path:  path,
				Name:  entry.Name(),
				Size:  info.Size(),
				IsDir: entry.IsDir(),
			})
		}

		return nil
	})

	return res, err
}
