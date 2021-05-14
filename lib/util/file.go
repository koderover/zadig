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

package util

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func GenerateTmpFile() (string, error) {
	var tmpFile *os.File

	tmpFile, err := ioutil.TempFile("", "")
	if err != nil {
		return "", err
	}

	_ = tmpFile.Close()

	return tmpFile.Name(), nil
}

func WriteFile(filename string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func Tar(src, dst string) (err error) {
	fw, err := os.Create(dst)
	if err != nil {
		return
	}
	defer fw.Close()

	gw := gzip.NewWriter(fw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(src, func(fileName string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = strings.TrimPrefix(fileName, string(filepath.Separator))

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		fr, err := os.Open(fileName)
		defer fr.Close()
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, fr)
		if err != nil {
			return err
		}
		return nil
	})
}

func UnTar(dst, src string) (err error) {
	fr, err := os.Open(src)
	if err != nil {
		return
	}
	defer fr.Close()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		return
	}
	defer gr.Close()

	// 通过 gr 创建 tar.Reader
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case hdr == nil:
			continue
		}

		dstFileDir := filepath.Join(dst, hdr.Name)

		switch hdr.Typeflag {
		case tar.TypeDir: // 如果是目录时候，创建目录
			if b := ExistDir(dstFileDir); !b {
				if err := os.MkdirAll(dstFileDir, 0775); err != nil {
					return err
				}
			}
		case tar.TypeReg: // 如果是文件就写入到磁盘
			file, err := os.OpenFile(dstFileDir, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(file, tr)
			if err != nil {
				return err
			}
			file.Close()
		}
	}

	return nil
}

func ExistDir(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}
