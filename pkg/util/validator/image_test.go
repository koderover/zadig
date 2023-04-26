/*
 * Copyright 2023 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsImageName(t *testing.T) {
	assert.False(t, IsValidImageName(""))
	assert.False(t, IsValidImageName("ubuntu：123"))
	assert.True(t, IsValidImageName("ubuntu"))
	assert.False(t, IsValidImageName("{ubuntu"))
	assert.False(t, IsValidImageName("ubuntu:"))
	assert.True(t, IsValidImageName("docker.io/ubuntu:123"))
	assert.True(t, IsValidImageName("ubuntu:latest"))
	assert.False(t, IsValidImageName("ubuntu{:latest"))
	assert.True(t, IsValidImageName("ubuntu:la-test"))
	assert.False(t, IsValidImageName("ubuntu:la-test/"))
	assert.True(t, IsValidImageName("kr.example.com/test/1/nginx:20060102150405"))
}
