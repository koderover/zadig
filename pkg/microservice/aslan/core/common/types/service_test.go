/*
Copyright 2023 The KodeRover Authors.

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

package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	"github.com/koderover/zadig/pkg/microservice/aslan/core/common/types"
)

var (
	kvs = []*types.ServiceVariableKV{
		{
			Key:   "strInt",
			Value: 1,
			Type:  types.ServiceVariableKVTypeString,
		},
		{
			Key:   "strStr",
			Value: "a string",
			Type:  types.ServiceVariableKVTypeString,
		},
		{
			Key:   "bool",
			Value: true,
			Type:  types.ServiceVariableKVTypeBoolean,
		},
		{
			Key:     "enum",
			Value:   11,
			Type:    types.ServiceVariableKVTypeEnum,
			Options: []string{"11", "22", "33"},
		},
		{
			Key: "yamlMap",
			Value: `
A: 1
B: a string`,
			Type: types.ServiceVariableKVTypeYaml,
		},
		{
			Key: "yamlArray",
			Value: `
- A: 1
  B: a string
- A: 2
  B: another string`,
			Type: types.ServiceVariableKVTypeYaml,
		},
		{
			Key:   "yamlStr",
			Value: "a string",
			Type:  types.ServiceVariableKVTypeYaml,
		},
	}

	yamlStr = `
strInt: 1
strStr: a string
bool: true
enum: 11
yamlStr: a string
yamlMap:
  A: 1
  B: a string
yamlArray:
- A: 1
  B: a string
- A: 2
  B: another string
`

	newYamlStr = `
strInt: 3
bool: true
enum: 11
yamlStr: a string
yamlMap:
- C: 1
  D: a string
- C: 2
  D: another string
yamlArray:
  X: 1
  Y: a string
`

	expectBaseOrigKVYaml = `
strInt: 3
bool: true
enum: 11
yamlStr: a string
yamlMap:
- C: 1
  D: a string
- C: 2
  D: another string
yamlArray:
  X: 1
  Y: a string
`

	floatKVs = []*types.ServiceVariableKV{
		{
			Key:     "floatEnum",
			Value:   1.5,
			Type:    types.ServiceVariableKVTypeEnum,
			Options: []string{"1.5", "2.5", "3.5"},
		},
	}
	floatOrigYamlStr = `
floatEnum: 1.5
`
	floatExpectedYamlStr = `
floatEnum: 3.5
`
)

var _ = Describe("Service", func() {
	var (
		kvToYamlTestCases []struct {
			kvs      []*types.ServiceVariableKV
			expected string
		}

		yamlToKVTestCases []struct {
			yamlStr  string
			origKVs  []*types.ServiceVariableKV
			expected string
		}
	)

	BeforeEach(func() {
		kvToYamlTestCases = []struct {
			kvs      []*types.ServiceVariableKV
			expected string
		}{
			{
				kvs:      kvs,
				expected: yamlStr,
			},
			{
				kvs:      floatKVs,
				expected: floatOrigYamlStr,
			},
		}

		yamlToKVTestCases = []struct {
			yamlStr  string
			origKVs  []*types.ServiceVariableKV
			expected string
		}{
			{
				yamlStr:  yamlStr,
				origKVs:  nil,
				expected: yamlStr,
			},
			{
				yamlStr:  newYamlStr,
				origKVs:  kvs,
				expected: expectBaseOrigKVYaml,
			},
			{
				yamlStr:  floatExpectedYamlStr,
				origKVs:  floatKVs,
				expected: floatExpectedYamlStr,
			},
		}
	})

	Context("convert kvs and yaml", func() {
		It("convert kvs to yaml", func() {
			for _, testCase := range kvToYamlTestCases {
				actual, err := types.ServiceVariableKVToYaml(testCase.kvs)
				Expect(err).ShouldNot(HaveOccurred())

				expectedMap := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(testCase.expected), &expectedMap)
				Expect(err).ShouldNot(HaveOccurred())

				expected, err := yaml.Marshal(expectedMap)
				Expect(err).ShouldNot(HaveOccurred())
				Expect(string(actual)).To(Equal(string(expected)))
			}
		})

		It("convert yaml to kvs", func() {
			for _, testCase := range yamlToKVTestCases {
				actual, err := types.YamlToServiceVariableKV(testCase.yamlStr, testCase.origKVs)
				Expect(err).ShouldNot(HaveOccurred())

				actualYaml, err := types.ServiceVariableKVToYaml(actual)
				Expect(err).ShouldNot(HaveOccurred())

				actualMap := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(actualYaml), &actualMap)
				Expect(err).ShouldNot(HaveOccurred())

				expectedMap := map[string]interface{}{}
				err = yaml.Unmarshal([]byte(testCase.expected), &expectedMap)
				Expect(err).ShouldNot(HaveOccurred())

				Expect(actualMap).To(Equal(expectedMap))
			}
		})
	})
})
