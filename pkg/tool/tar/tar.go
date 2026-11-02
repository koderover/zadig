/*
Copyright 2024 The KodeRover Authors.

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

package ssh

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Untar(tarFile, destDir string, gzipCompressed bool) error {
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := os.Open(tarFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var tarReader *tar.Reader
	if gzipCompressed {
		gzipReader, err := gzip.NewReader(file)
		if err != nil {
			return err
		}
		defer gzipReader.Close()
		tarReader = tar.NewReader(gzipReader)
	} else {
		tarReader = tar.NewReader(file)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if !filepath.IsLocal(header.Name) {
			return fmt.Errorf("unsafe archive path: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(header.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := root.MkdirAll(filepath.Dir(header.Name), 0755); err != nil {
				return err
			}
			outFile, err := root.OpenFile(header.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		default:
			fmt.Printf("Unable to untar type: %c in file %s", header.Typeflag, header.Name)
		}
	}
	return nil
}
