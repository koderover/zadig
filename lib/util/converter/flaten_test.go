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

package converter_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/lib/util/converter"
)

type testParams struct {
	yaml     string
	expected map[string]interface{}
}

var testData1 = `
a: b
c:
  d: e
`

var testData2 = `
a: b
c:
  - d: e
  - d: f
`

var testData3 = `
a: b
c:
  - d
  - e
`

var _ = Describe("Testing flatten", func() {

	DescribeTable("Testing YamlToFlatMap",
		func(p testParams) {
			res, err := converter.YamlToFlatMap([]byte(p.yaml))

			Expect(err).ShouldNot(HaveOccurred())
			Expect(res).To(Equal(p.expected))
		},
		Entry("yaml without array value", testParams{
			yaml:     testData1,
			expected: map[string]interface{}{"a": "b", "c.d": "e"},
		}),
		Entry("yaml with nested array", testParams{
			yaml:     testData2,
			expected: map[string]interface{}{"a": "b", "c[0].d": "e", "c[1].d": "f"},
		}),
		Entry("yaml with string array", testParams{
			yaml:     testData3,
			expected: map[string]interface{}{"a": "b", "c[0]": "d", "c[1]": "e"},
		}),
	)
})
