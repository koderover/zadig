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

package fs

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func RelativeToCurrentPath(path string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	return filepath.Rel(dir, path)
}

// ShortenFileBase removes baseDir from fullPath (but keep the last element of baseDir).
// e.g. ShortenFileBase("a/b", "a/b/c.go") => "b/c.go"
func ShortenFileBase(baseDir, fullPath string) string {
	if baseDir == "" || baseDir == "." {
		return fullPath
	}
	if baseDir == "/" {
		return strings.Trim(fullPath, "/")
	}
	baseDir = strings.Trim(baseDir, string(filepath.Separator))

	base := filepath.Base(baseDir)
	res := strings.SplitN(fullPath, baseDir+string(filepath.Separator), 2)
	if len(res) == 2 {
		return filepath.Join(base, res[1])
	}

	return fullPath
}

// Tar archives the src file system and saves to disk with path dst.
// src file system is a tree of files from disk, memory or any other places which implement fs.FS.
func Tar(src fs.FS, dst string) error {
	dir := filepath.Dir(dst)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	fw, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		err = fw.Close()
	}()

	gw := gzip.NewWriter(fw)
	defer func() {
		err = gw.Close()
	}()

	tw := tar.NewWriter(gw)
	defer func() {
		err = tw.Close()
	}()

	return fs.WalkDir(src, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == "." {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			return err
		}
		hdr.Name = strings.TrimPrefix(path, string(filepath.Separator))

		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !entry.Type().IsRegular() {
			return nil
		}

		fr, err := src.Open(path)
		if err != nil {
			return err
		}
		defer fr.Close()

		_, err = io.Copy(tw, fr)

		return err
	})
}

// Untar extracts the src tarball and saves to disk with path dst.
func Untar(src, dst string) (err error) {
	fr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = fr.Close()
	}()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return err
	}
	defer func() {
		err = gr.Close()
	}()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if hdr == nil {
			continue
		}

		dirOrFile := filepath.Join(dst, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err = os.MkdirAll(dirOrFile, 0775); err != nil {
				return err
			}
		case tar.TypeReg:
			file, err := os.OpenFile(dirOrFile, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(file, tr)
			_ = file.Close()
			if err != nil {
				return err
			}
		}
	}
}

// SaveToDisk saves the file tree to disk with path root.
// file tree is a tree of files from disk, memory or any other places which implement fs.FS.
func SaveToDisk(fileTree fs.FS, root string) (err error) {
	return fs.WalkDir(fileTree, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		dirOrFile := filepath.Join(root, path)
		mode := entry.Type()
		switch {
		case mode.IsDir():
			return os.MkdirAll(dirOrFile, 0775)
		case mode.IsRegular():
			contents, err := fs.ReadFile(fileTree, path)
			if err != nil {
				return err
			}

			// TODO: fix the perm mode
			return os.WriteFile(dirOrFile, contents, 0644)
		default:
			return nil
		}
	})
}
