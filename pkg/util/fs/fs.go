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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func FileExists(filePath string) (bool, error) {
	st, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}

	if st.IsDir() {
		return false, fmt.Errorf("%s is a directory", filePath)
	}

	return true, nil
}

func SaveFile(src io.ReadCloser, dst string) error {
	// Verify if destination already exists.
	st, err := os.Stat(dst)

	// If the destination exists and is a directory.
	if err == nil && st.IsDir() {
		return errors.New("fileName is a directory")
	}

	// Extract top level directory.
	objectDir, _ := filepath.Split(dst)
	if objectDir != "" {
		// Create any missing top level directories.
		if err := os.MkdirAll(objectDir, 0700); err != nil {
			return err
		}
	}

	// remove the file in case truncate fails
	err = os.Remove(dst)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create a new file.
	file, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, src)
	return err
}
