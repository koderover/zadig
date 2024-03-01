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

package reaper

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoCacheManager_ArchiveAndUnArchive(t *testing.T) {
	workspace, err := ioutil.TempDir(os.TempDir(), "reaper")
	assert.Nil(t, err)

	f, err := os.OpenFile(filepath.Join(workspace, "readme.md"), os.O_CREATE|os.O_RDWR, 0755)
	assert.Nil(t, err)
	_, err = f.Write([]byte("hello world"))
	assert.Nil(t, err)
	assert.Nil(t, f.Close())

	cacheMgr := NewGoCacheManager()
	dest := filepath.Join(workspace, "reaper.tar.gz")
	assert.Nil(t, cacheMgr.Archive(workspace, dest))

	assert.Nil(t, cacheMgr.Unarchive(dest, workspace))
}
