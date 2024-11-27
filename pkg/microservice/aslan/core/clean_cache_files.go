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

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	commonconfig "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func cleanCacheFiles() {
	projectPath := filepath.Join(commonconfig.DataPath(), "project")
	err := cleanProjectFiles(projectPath)
	if err != nil {
		log.Errorf("[cleanCacheFiles] failed to clean project cache files: %v", err)
	}
}

func cleanProjectFiles(path string) error {
	return traverseDir(path, cleanWorkflowFiles)
}

func cleanWorkflowFiles(path string, info os.FileInfo) error {
	workflowPath := filepath.Join(path, "workflow")
	return traverseDir(workflowPath, cleanTaskFiles)
}

func cleanjobTaskFiles(path string, info os.FileInfo) error {
	jobTaskPath := filepath.Join(path, "jobTask")
	return traverseDir(jobTaskPath, cleanTaskFiles)
}

func cleanTaskFiles(path string, info os.FileInfo) error {
	taskPath := filepath.Join(path, "task")
	return traverseDir(taskPath, cleanHtmlReportFiles)
}

func cleanHtmlReportFiles(path string, info os.FileInfo) error {
	htmlReportPath := filepath.Join(path, "html-report")
	htmlReportInfo, err := os.Stat(htmlReportPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat htmlReportPath: %s, err: %v", htmlReportPath, err)
	}

	if time.Since(htmlReportInfo.ModTime()) > 7*24*time.Hour {
		err = os.RemoveAll(htmlReportPath)
		if err != nil {
			return fmt.Errorf("failed to remove htmlReportPath: %s, err: %v", htmlReportPath, err)
		}
	}
	return nil
}

func traverseDir(path string, fn func(string, os.FileInfo) error) error {
	pathInfo, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat path: %s, err: %v", path, err)
	}

	if !pathInfo.IsDir() {
		return fmt.Errorf("path is not a directory: %s", path)
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory: %s, err: %v", path, err)
	}

	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info: %s, err: %v", file.Name(), err)
		}
		if info.IsDir() {
			err = fn(filepath.Join(path, file.Name()), info)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
