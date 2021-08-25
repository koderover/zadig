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

package fs_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/pkg/util/fs"
)

type testParams struct {
	base, path, expectedPath string
}

var _ = Describe("Testing file", func() {

	DescribeTable("Testing ShortenFileBase",
		func(p testParams) {
			path := fs.ShortenFileBase(p.base, p.path)
			Expect(path).To(Equal(p.expectedPath))
		},
		Entry("short path", testParams{
			base:         "a",
			path:         "a/b/c.go",
			expectedPath: "a/b/c.go",
		}),
		Entry("short path with '/'", testParams{
			base:         "a/",
			path:         "a/b/c.go",
			expectedPath: "a/b/c.go",
		}),
		Entry("long path", testParams{
			base:         "a/b",
			path:         "a/b/c.go",
			expectedPath: "b/c.go",
		}),
		Entry("long path with '/'", testParams{
			base:         "a/b/",
			path:         "a/b/c.go",
			expectedPath: "b/c.go",
		}),
		Entry("longer path", testParams{
			base:         "a/d/b",
			path:         "a/d/b/c.go",
			expectedPath: "b/c.go",
		}),
		Entry("empty path", testParams{
			base:         "",
			path:         "b/c.go",
			expectedPath: "b/c.go",
		}),
		Entry("current path", testParams{
			base:         ".",
			path:         "b/c.go",
			expectedPath: "b/c.go",
		}),
		Entry("root path", testParams{
			base:         "/",
			path:         "/b/c.go",
			expectedPath: "b/c.go",
		}),
	)
})

func ExampleShortenFileBase() {
	fmt.Println(fs.ShortenFileBase("a", "a/b/c.go"))
	fmt.Println(fs.ShortenFileBase("a/", "a/b/c.go"))
	fmt.Println(fs.ShortenFileBase("a/b", "a/b/c.go"))
	fmt.Println(fs.ShortenFileBase("a/b/", "a/b/c.go"))
	fmt.Println(fs.ShortenFileBase("a/d/b", "a/d/b/c.go"))
	fmt.Println(fs.ShortenFileBase("", "b/c.go"))
	fmt.Println(fs.ShortenFileBase(".", "b/c.go"))
	fmt.Println(fs.ShortenFileBase("/", "/b/c.go"))

	//Output:
	//
	//a/b/c.go
	//a/b/c.go
	//b/c.go
	//b/c.go
	//b/c.go
	//b/c.go
	//b/c.go
	//b/c.go
}
