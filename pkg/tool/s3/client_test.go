/*
Copyright 2022 The KodeRover Authors.

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

package s3

import (
	"testing"
)

func TestDetectMimetype(t *testing.T) {
	type testcase struct {
		file     string
		mimetype string
	}
	testcases := []testcase{
		{"", ""},
		{"/a/abc.", ""},
		{"abc/ab", ""},
		{".abc.a", ""},
		{"./abc.js", "application/javascript"},
		{"./abc.css", "text/css; charset=utf-8"},
		{"./abc.html", "text/html; charset=utf-8"},
	}
	for _, ts := range testcases {
		m := detectMimetype(ts.file)
		if m != ts.mimetype {
			t.Errorf("Expected result for path <%s> to be <%s> but got <%s>", ts.file, ts.mimetype, m)
		}
	}
}
