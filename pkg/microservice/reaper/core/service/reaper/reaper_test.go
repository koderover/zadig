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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mholt/archiver"
	"github.com/stretchr/testify/assert"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	_ "github.com/koderover/zadig/pkg/util/testing"
)

func TestReaper_CompressAndDecompressCache(t *testing.T) {
	testReaperCompressAndDecompressCache(t, NewGoCacheManager())
}

func createReaperForTest(t *testing.T, cm CacheManager) *Reaper {
	workspace, err := ioutil.TempDir("", "reaper")
	assert.Nil(t, err)

	r := &Reaper{
		Ctx: &meta.Context{
			Workspace: workspace,
		},

		cm: cm,
	}
	return r
}

func testReaperCompressAndDecompressCache(t *testing.T, manager CacheManager) {
	r := createReaperForTest(t, manager)

	err := r.EnsureActiveWorkspace("")
	assert.Nil(t, err)
	t.Logf("temp workspace: %s", r.ActiveWorkspace)

	assert.True(t, strings.Index(r.GetCacheFile(), "reaper.tar.gz") > 0)
	t.Logf("cache file: %s", r.GetCacheFile())

	f, err := os.OpenFile(filepath.Join(r.ActiveWorkspace, "readme.md"), os.O_CREATE|os.O_RDWR, 0755)
	assert.Nil(t, err)

	_, err = f.Write([]byte("hello world"))
	assert.Nil(t, err)
	assert.Nil(t, f.Close())

	f, err = os.Create(filepath.Join(r.ActiveWorkspace, "readme2.md"))
	assert.Nil(t, err)

	err = os.MkdirAll(filepath.Join(r.ActiveWorkspace, "readme"), os.ModePerm)
	assert.Nil(t, err)

	_, err = f.Write([]byte("hello world 2"))
	assert.Nil(t, err)
	assert.Nil(t, f.Close())

	assert.Nil(t, r.CompressCache(""))

	// workspace should be removed after compress
	_, err = os.Stat(r.ActiveWorkspace)
	assert.True(t, os.IsNotExist(err))

	// decompress
	assert.Nil(t, r.DecompressCache())

	_, err = os.Stat(r.ActiveWorkspace)
	assert.Nil(t, err)

	content, err := ioutil.ReadFile(filepath.Join(r.ActiveWorkspace, "readme.md"))
	assert.Nil(t, err)
	assert.Equal(t, "hello world", string(content))

	content, err = ioutil.ReadFile(filepath.Join(r.ActiveWorkspace, "readme2.md"))
	assert.Nil(t, err)
	assert.Equal(t, "hello world 2", string(content))

	// compress again should override
	f, err = os.Create(filepath.Join(r.ActiveWorkspace, "readme3.md"))
	assert.Nil(t, err)

	_, err = f.Write([]byte("hello world 3"))
	assert.Nil(t, f.Close())
	assert.Nil(t, err)

	assert.Nil(t, r.CompressCache(""))

	var found bool
	var permissionKept bool

	_ = archiver.Walk(
		r.GetCacheFile(),
		func(f archiver.File) error {
			if f.Name() == "readme3.md" {
				found = f.Mode().Perm() == os.FileMode(0644)
			} else if f.Name() == "readme.md" {
				permissionKept = f.Mode().Perm() == os.FileMode(0755)
			}
			return nil
		},
	)

	assert.True(t, found)
	assert.True(t, permissionKept)

	assert.Nil(t, r.EnsureActiveWorkspace(""))
	assert.Nil(t, r.DecompressCache())
	assert.Nil(t, r.CompressCache(""))
}

func TestReaperMaskSecretEnvs(t *testing.T) {
	const (
		secretEnvKey = "MY_SECRET"
		secretEnvVal = "my_secret_value"
	)
	r := createReaperForTest(t, NewGoCacheManager())
	r.Ctx.SecretEnvs = []string{fmt.Sprintf("%s=%s", secretEnvKey, secretEnvVal)}

	out := r.maskSecretEnvs(fmt.Sprintf("%s=%s", secretEnvKey, secretEnvVal))
	assert.Equal(t, fmt.Sprintf("%s=%s", secretEnvKey, secretEnvMask), out)
}
