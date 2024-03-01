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

package errors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrors(t *testing.T) {
	assert := assert.New(t)

	httpErr := NewHTTPError(400, "testErr", "error description")
	assert.Equal(400, httpErr.Code())
	assert.Equal("testErr", httpErr.Error())
	assert.Equal("error description", httpErr.Desc())

	httpErr.AddDesc("error description updated")
	assert.Equal("error description updated", httpErr.Desc())

	err2 := NewWithDesc(httpErr, "new error with desc")
	tmp2, ok := err2.(*HTTPError)
	assert.True(ok)
	assert.Equal("new error with desc", tmp2.Desc())

	extras := map[string]interface{}{
		"key": "extra",
	}
	err3 := NewWithExtras(httpErr, "new error with extras", extras)
	tmp3, ok := err3.(*HTTPError)
	assert.True(ok)
	assert.Equal("new error with extras", tmp3.Desc())
	assert.Equal(extras, tmp3.Extra())

	code, message := ErrorMessage(httpErr)
	assert.Equal(400, code)
	assert.Equal(400, message["code"])
	assert.Equal(httpErr.Error(), message["message"])
	assert.Equal(httpErr.Desc(), message["description"])
}
