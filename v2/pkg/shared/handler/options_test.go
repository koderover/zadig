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
	"github.com/onsi/ginkgo/v2"
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
	ginkgo.DescribeTable("Testing with different structs",
		func(p testCase) {
			result := responseHelper(p.input)

			Expect(result).To(Equal(p.expect))
		},
		ginkgo.Entry("primitive type -- int", testCase{
			input:  1,
			expect: 1,
		}),
		ginkgo.Entry("primitive type -- string", testCase{
			input:  mystring,
			expect: mystring,
		}),
		ginkgo.Entry("primitive type -- bool", testCase{
			input:  true,
			expect: true,
		}),
		ginkgo.Entry("primitive type -- byte", testCase{
			input:  byte('a'),
			expect: byte('a'),
		}),
		ginkgo.Entry("primitive type -- rune", testCase{
			input:  'a',
			expect: 'a',
		}),
		ginkgo.Entry("primitive type -- float", testCase{
			input:  12.34,
			expect: 12.34,
		}),
		ginkgo.Entry("slice -- string -- unchanged", testCase{
			input:  []string{"a"},
			expect: &[]string{"a"},
		}),
		ginkgo.Entry("slice -- string -- changed", testCase{
			input:  nilSlice,
			expect: &[]string{},
		}),
		ginkgo.Entry("slice -- slice -- unchanged", testCase{
			input:  [][]string{{"a"}},
			expect: &[][]string{{"a"}},
		}),
		ginkgo.Entry("slice -- slice -- changed", testCase{
			input:  nil2dSlice,
			expect: &[][]string{},
		}),
		ginkgo.Entry("slice -- pointer -- unchanged", testCase{
			input:  []*string{&mystring},
			expect: &[]*string{&mystring},
		}),
		ginkgo.Entry("slice -- pointer -- changed", testCase{
			input:  nilSlicetoPointer,
			expect: &[]*string{},
		}),
		ginkgo.Entry("slice -- map -- unchanged", testCase{
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
		ginkgo.Entry("slice -- map -- changed", testCase{
			input:  nilSlicetoMap,
			expect: &[]map[string]interface{}{},
		}),
		ginkgo.Entry("slice in a struct -- unchanged", testCase{
			input:  StructWithNormalSlice{Myarr: []string{"a"}},
			expect: &StructWithNormalSlice{Myarr: []string{"a"}},
		}),
		ginkgo.Entry("slice in a struct -- changed", testCase{
			input:  StructWithNormalSlice{Myarr: nil},
			expect: &StructWithNormalSlice{Myarr: []string{}},
		}),
		ginkgo.Entry("slice in a map -- unchanged", testCase{
			input: map[string][]string{
				"a": []string{"lol"},
			},
			expect: &map[string][]string{
				"a": []string{"lol"},
			},
		}),
		ginkgo.Entry("slice in a map -- changed", testCase{
			input: map[string][]string{
				"a": nil,
			},
			expect: &map[string][]string{
				"a": []string{},
			},
		}),
		ginkgo.Entry("pointer to a struct -- unchanged", testCase{
			input:  &StructWithNormalSlice{Myarr: []string{"a"}},
			expect: &StructWithNormalSlice{Myarr: []string{"a"}},
		}),
		ginkgo.Entry("pointer to a struct -- changed", testCase{
			input:  &StructWithNormalSlice{Myarr: nilSlice},
			expect: &StructWithNormalSlice{Myarr: []string{}},
		}),
		ginkgo.Entry("pointer to a slice -- unchanged", testCase{
			input:  &[]string{"a"},
			expect: &[]string{"a"},
		}),
		ginkgo.Entry("pointer to a slice -- changed", testCase{
			input:  &nilSlice,
			expect: &[]string{},
		}),
		ginkgo.Entry("pointer to slice in a map -- unchanged", testCase{
			input: &map[string][]string{
				"a": []string{"lol"},
			},
			expect: &map[string][]string{
				"a": []string{"lol"},
			},
		}),
		ginkgo.Entry("pointer to slice in a map -- changed", testCase{
			input: &map[string][]string{
				"a": nil,
			},
			expect: &map[string][]string{
				"a": []string{},
			},
		}),
		ginkgo.Entry("slice of struct in a struct -- unchanged", testCase{
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
		ginkgo.Entry("slice of struct in a struct -- changed", testCase{
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
		ginkgo.Entry("slice of pointers in struct -- unchanged", testCase{
			input: StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{"a"}}}},
			expect: &StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{"a"}}}},
		}),
		ginkgo.Entry("slice of pointers in struct -- changed", testCase{
			input: StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{nil}}},
			expect: &StructWithSliceOfPointer{
				InnerStruct: []*StructWithNormalSlice{
					&StructWithNormalSlice{[]string{}}}},
		}),
		ginkgo.Entry("slice of pointers in struct -- unchanged", testCase{
			input: StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{"a"}}}},
			expect: &StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{"a"}}}},
		}),
		ginkgo.Entry("slice of pointers in struct -- changed", testCase{
			input: StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{nil}}},
			expect: &StructWithPointerToSlice{
				InnerStruct: &[]StructWithNormalSlice{
					StructWithNormalSlice{[]string{}}}},
		}),
		ginkgo.Entry("slice of pointer in struct -- nil pointer", testCase{
			input:  StructWithPointerToSlice{InnerStruct: nilPointerToSlice},
			expect: &StructWithPointerToSlice{InnerStruct: &[]StructWithNormalSlice{}},
		}),
		ginkgo.Entry("slice of pointer of slice -- unchanged", testCase{
			input: []*[]string{
				&[]string{"a"},
			},
			expect: &[]*[]string{
				&[]string{"a"},
			},
		}),
		ginkgo.Entry("slice of pointer of slice -- nil", testCase{
			input: []*[]StructWithNormalSlice{
				nilPointerToSlice,
			},
			expect: &[]*[]StructWithNormalSlice{
				&[]StructWithNormalSlice{},
			},
		}),
		ginkgo.Entry("struct with nil pointer", testCase{
			input:  StructWithPointer{Myptr: nil},
			expect: &StructWithPointer{Myptr: nil},
		}),
		ginkgo.Entry("struct with nil interface", testCase{
			input:  StructWithInterface{InnerStuff: nil},
			expect: &StructWithInterface{InnerStuff: nil},
		}),
		ginkgo.Entry("struct with interface -- string", testCase{
			input:  StructWithInterface{InnerStuff: "a"},
			expect: &StructWithInterface{InnerStuff: "a"},
		}),
		ginkgo.Entry("struct with interface -- nil slice", testCase{
			input:  StructWithInterface{InnerStuff: nilSlice},
			expect: &StructWithInterface{InnerStuff: []string{}},
		}),
		ginkgo.Entry("struct with interface -- non-nil slice", testCase{
			input:  StructWithInterface{InnerStuff: []string{"a"}},
			expect: &StructWithInterface{InnerStuff: []string{"a"}},
		}),
		ginkgo.Entry("struct with interface -- nil struct", testCase{
			input:  StructWithInterface{InnerStuff: nilStruct},
			expect: &StructWithInterface{InnerStuff: nilStruct},
		}),
		ginkgo.Entry("struct with interface -- non-nil struct", testCase{
			input: StructWithInterface{InnerStuff: struct {
				A string
			}{"a"}},
			expect: &StructWithInterface{InnerStuff: struct {
				A string
			}{"a"}},
		}),
	)
})
