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

package types

import (
	"os"
)

// FileInfo ...
type FileInfo struct {
	// parent path of the file
	Parent string `json:"parent"`
	// base name of the file
	Name string `json:"name"`
	// length in bytes for regular files; system-dependent for others
	Size int64 `json:"size"`
	// file mode bits
	Mode os.FileMode `json:"mode"`
	// modification time
	ModTime int64 `json:"mod_time"`
	// abbreviation for Mode().IsDir()
	IsDir bool `json:"is_dir"`
}
