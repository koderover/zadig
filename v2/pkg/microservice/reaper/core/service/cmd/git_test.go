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

package cmd

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitGit(t *testing.T) {
	assert := assert.New(t)

	cmd := InitGit(os.TempDir())
	assert.Equal(os.TempDir(), cmd.Dir)
	assert.Len(cmd.Args, 2)
}

func TestRemote(t *testing.T) {
	assert := assert.New(t)

	cmd := RemoteAdd("origin", "demo")
	assert.Len(cmd.Args, 5)
	assert.Equal("origin", cmd.Args[3])
	assert.Equal("demo", cmd.Args[4])
}

func TestCheckoutHead(t *testing.T) {
	assert := assert.New(t)

	cmd := CheckoutHead()
	assert.Len(cmd.Args, 4)
}

func TestFetch(t *testing.T) {
	assert := assert.New(t)

	cmd := Fetch("origin", "refs/heads/master")
	assert.Len(cmd.Args, 5)
	assert.Equal("origin", cmd.Args[2])
	assert.Equal("+refs/heads/master", cmd.Args[3])
	assert.Equal("--depth=1", cmd.Args[4])
}

func TestMerge(t *testing.T) {
	assert := assert.New(t)

	cmd := Merge("master")
	assert.Len(cmd.Args, 4)
	assert.Equal("master", cmd.Args[2])
}
func TestUpdateSubmodules(t *testing.T) {
	assert := assert.New(t)

	cmd := UpdateSubmodules()
	assert.Len(cmd.Args, 5)
}

func TestSetConfig(t *testing.T) {
	assert := assert.New(t)

	cmd := SetConfig("user.email", "testuser")
	assert.Len(cmd.Args, 5)
	assert.Equal("user.email", cmd.Args[3])
	assert.Equal("testuser", cmd.Args[4])
}

func TestShowLastLog(t *testing.T) {
	assert := assert.New(t)

	cmd := ShowLastLog()
	assert.Len(cmd.Args, 5)
}
