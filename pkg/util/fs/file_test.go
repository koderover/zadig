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
	"archive/tar"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/v2/pkg/util/fs"
)

type testParams struct {
	base, path, expectedPath string
}

var _ = Describe("Testing file", func() {
	It("round-trips tar.gz contents and permissions", func() {
		tempDir := GinkgoT().TempDir()
		source := filepath.Join(tempDir, "source")
		Expect(os.MkdirAll(source, 0755)).To(Succeed())
		filePath := filepath.Join(source, "script.sh")
		Expect(os.WriteFile(filePath, []byte("echo secure"), 0755)).To(Succeed())

		archivePath := filepath.Join(tempDir, "cache.tar.gz")
		Expect(fs.Tar(os.DirFS(source), archivePath)).To(Succeed())
		destination := filepath.Join(tempDir, "destination")
		Expect(fs.Untar(archivePath, destination)).To(Succeed())

		contents, err := os.ReadFile(filepath.Join(destination, "script.sh"))
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).To(Equal([]byte("echo secure")))
		info, err := os.Stat(filepath.Join(destination, "script.sh"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0755)))
	})

	It("rejects archive entries that escape the destination", func() {
		tempDir := GinkgoT().TempDir()
		archivePath := filepath.Join(tempDir, "malicious.tar.gz")
		archiveFile, err := os.Create(archivePath)
		Expect(err).NotTo(HaveOccurred())
		gzipWriter := gzip.NewWriter(archiveFile)
		tarWriter := tar.NewWriter(gzipWriter)
		contents := []byte("escaped")
		Expect(tarWriter.WriteHeader(&tar.Header{
			Name: "../escaped.txt",
			Mode: 0644,
			Size: int64(len(contents)),
		})).To(Succeed())
		_, err = tarWriter.Write(contents)
		Expect(err).NotTo(HaveOccurred())
		Expect(tarWriter.Close()).To(Succeed())
		Expect(gzipWriter.Close()).To(Succeed())
		Expect(archiveFile.Close()).To(Succeed())

		destination := filepath.Join(tempDir, "destination")
		Expect(fs.Untar(archivePath, destination)).To(MatchError(ContainSubstring("escapes destination")))
		_, err = os.Stat(filepath.Join(tempDir, "escaped.txt"))
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

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
