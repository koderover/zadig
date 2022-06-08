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

package gerrit

import "testing"

func TestKeywordToRegexp(t *testing.T) {
	type testcase struct {
		keyword string
		result  string
	}
	testcases := []testcase{
		{"", ".*"},
		{" ", ".*"},
		{"  ", ".*"},
		{"ab", ".*ab.*"},
		{"   a     b  ", ".*a.*b.*"},
		{"   a b  c  d   ", ".*a.*b.*c.*d.*"},
		{"   a/b c-d", ".*a/b.*c-d.*"},
	}

	for _, ts := range testcases {
		regstr := keywordToRegexp(ts.keyword)
		if regstr != ts.result {
			t.Errorf("Expected result for keyword <%s> to be <%s> but got <%s>", ts.keyword, ts.result, regstr)
		}
	}
}
