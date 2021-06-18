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

package handler

import (
	"github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type testCase struct {
	input  interface{}
	expect interface{}
}

type StructWithPointer struct {
	Myptr *string
}

type StructWithNormalSlice struct {
	Myarr []string
}

type StructWithStructSlice struct {
	InnerStruct []StructWithNormalSlice
}

type StructWithSliceOfPointer struct {
	InnerStruct []*StructWithNormalSlice
}

type StructWithPointerToSlice struct {
	InnerStruct *[]StructWithNormalSlice
}

type StructWithInterface struct {
	InnerStuff interface{}
}

var _ = ginkgo.Describe("Replace nil Slice With Empty Slice", func() {
	mystring := "a"
	var nilSlice []string
	var nil2dSlice [][]string
	var nilSlicetoPointer []*string
	var nilSlicetoMap []map[string]interface{}
	var nilPointerToSlice *[]StructWithNormalSlice
	var nilStruct StructWithInterface
	DescribeTable("Testing with different structs",
		func(p testCase) {
			result := responseHelper(p.input)

			Expect(result).To(Equal(p.expect))
		},
		Entry("primitive type -- int", testCase{
			input:  1,
			expect: 1,
		}),
		Entry("primitive type -- string", testCase{
			input:  mystring,
			expect: mystring,
		}),
		Entry("primitive type -- bool", testCase{
			input:  true,
			expect: true,
		}),
		Entry("primitive type -- byte", testCase{
			input:  byte('a'),
			expect: byte('a'),
		}),
		Entry("primitive type -- rune", testCase{
			input:  'a',
			expect: 'a',
		}),
		Entry("primitive type -- float", testCase{
			input:  12.34,
			expect: 12.34,
		}),
		Entry("slice -- string -- unchanged", testCase{
			input:  []string{"a"},
			expect: &[]string{"a"},
		}),
		Entry("slice -- string -- changed", testCase{
			input:  nilSlice,
			expect: &[]string{},
		}),
		Entry("slice -- slice -- unchanged", testCase{
			input:  [][]string{{"a"}},
			expect: &[][]string{{"a"}},
		}),
		Entry("slice -- slice -- changed", testCase{
			input:  nil2dSlice,
			expect: &[][]string{},
		}),
		Entry("slice -- pointer -- unchanged", testCase{
			input:  []*string{&mystring},
			expect: &[]*string{&mystring},
		}),
		Entry("slice -- pointer -- changed", testCase{
			input:  nilSlicetoPointer,
			expect: &[]*string{},
		}),
		Entry("slice -- map -- unchanged", testCase{
			input: []map[string]interface{}{
				map[string]interface{}{
					"a": "b",
				},
			},
			expect: &[]map[string]interface{}{
				map[string]interface{}{
					"a": "b",
				},
			},
		}),
		Entry("slice -- map -- changed", testCase{
			input:  nilSlicetoMap,
			expect: &[]map[string]interface{}{},
		}),
		Entry("slice in a struct -- unchanged", testCase{
			input:  StructWithNormalSlice{Myarr: []string{"a"}},
			expect: &StructWithNormalSlice{Myarr: []string{"a"}},
		}),
		Entry("slice in a struct -- changed", testCase{
			input:  StructWithNormalSlice{Myarr: nil},
			expect: &StructWithNormalSlice{Myarr: []string{}},
		}),
		Entry("slice in a map -- unchanged", testCase{
			input: map[string][]string{
				"a": []string{"lol"},
			},
			expect: &map[string][]string{
				"a": []string{"lol"},
			},
		}),
		Entry("slice in a map -- changed", testCase{
			input: map[string][]string{
				"a": nil,
			},
			expect: &map[string][]string{
				"a": []string{},
			},
		}),
		Entry("pointer to a struct -- unchanged", testCase{
			input:  &StructWithNormalSlice{Myarr: []string{"a"}},
			expect: &StructWithNormalSlice{Myarr: []string{"a"}},
		}),
		Entry("pointer to a struct -- changed", testCase{
			input:  &StructWithNormalSlice{Myarr: nilSlice},
			expect: &StructWithNormalSlice{Myarr: []string{}},
		}),
		Entry("pointer to a slice -- unchanged", testCase{
			input:  &[]string{"a"},
			expect: &[]string{"a"},
		}),
		Entry("pointer to a slice -- changed", testCase{
			input:  &nilSlice,
			expect: &[]string{},
		}),
		Entry("pointer to slice in a map -- unchanged", testCase{
			input: &map[string][]string{
				"a": []string{"lol"},
			},
			expect: &map[string][]string{
				"a": []string{"lol"},
			},
		}),
		Entry("pointer to slice in a map -- changed", testCase{
			input: &map[string][]string{
				"a": nil,
			},
			expect: &map[string][]string{
				"a": []string{},
			},
		}),
		Entry("slice of struct in a struct -- unchanged", testCase{
			input: StructWithStructSlice{InnerStruct: []StructWithNormalSlice{
				{
					Myarr: []string{"a"},
				},
			}},
			expect: &StructWithStructSlice{InnerStruct: []StructWithNormalSlice{
				{
					Myarr: []string{"a"},
				},
			}},
		}),
		Entry("slice of struct in a struct -- changed", testCase{
			input: StructWithStructSlice{InnerStruct: []StructWithNormalSlice{
				{
					Myarr: nilSlice,
				},
			}},
			expect: &StructWithStructSlice{InnerStruct: []StructWithNormalSlice{
				{
					Myarr: []string{},
				},
			}},
		}),
		Entry("slice of pointers in struct -- unchanged", testCase{
			input: StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{"a"}}}},
			expect: &StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{"a"}}}},
		}),
		Entry("slice of pointers in struct -- changed", testCase{
			input: StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{nil}}},
			expect: &StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{}}}},
		}),
		Entry("slice of pointers in struct -- unchanged", testCase{
			input: StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{"a"}}}},
			expect: &StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{"a"}}}},
		}),
		Entry("slice of pointers in struct -- changed", testCase{
			input: StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{nil}}},
			expect: &StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{}}}},
		}),
		Entry("slice of pointer in struct -- nil pointer", testCase{
			input:  StructWithPointerToSlice{InnerStruct: nilPointerToSlice},
			expect: &StructWithPointerToSlice{InnerStruct: &[]StructWithNormalSlice{}},
		}),
		Entry("slice of pointer of slice -- unchanged", testCase{
			input: []*[]string{
				&[]string{"a"},
			},
			expect: &[]*[]string{
				&[]string{"a"},
			},
		}),
		Entry("slice of pointer of slice -- nil", testCase{
			input: []*[]StructWithNormalSlice{
				nilPointerToSlice,
			},
			expect: &[]*[]StructWithNormalSlice{
				&[]StructWithNormalSlice{},
			},
		}),
		Entry("struct with nil pointer", testCase{
			input:  StructWithPointer{Myptr: nil},
			expect: &StructWithPointer{Myptr: nil},
		}),
		Entry("struct with nil interface", testCase{
			input:  StructWithInterface{InnerStuff: nil},
			expect: &StructWithInterface{InnerStuff: nil},
		}),
		Entry("struct with interface -- string", testCase{
			input:  StructWithInterface{InnerStuff: "a"},
			expect: &StructWithInterface{InnerStuff: "a"},
		}),
		Entry("struct with interface -- nil slice", testCase{
			input:  StructWithInterface{InnerStuff: nilSlice},
			expect: &StructWithInterface{InnerStuff: []string{}},
		}),
		Entry("struct with interface -- non-nil slice", testCase{
			input:  StructWithInterface{InnerStuff: []string{"a"}},
			expect: &StructWithInterface{InnerStuff: []string{"a"}},
		}),
		Entry("struct with interface -- nil struct", testCase{
			input:  StructWithInterface{InnerStuff: nilStruct},
			expect: &StructWithInterface{InnerStuff: nilStruct},
		}),
		Entry("struct with interface -- non-nil struct", testCase{
			input: StructWithInterface{InnerStuff: struct {
				A string
			}{"a"}},
			expect: &StructWithInterface{InnerStuff: struct {
				A string
			}{"a"}},
		}),
	)
})
