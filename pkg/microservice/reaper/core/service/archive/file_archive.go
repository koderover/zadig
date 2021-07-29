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

package archive

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/microservice/reaper/internal/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
)

type WorkspaceAchiever struct {
	paths        []string
	gitFolders   []string
	wd           string
	files        map[string]os.FileInfo
	StorageURI   string
	PipelineName string
	ServiceName  string
}

func NewWorkspaceAchiever(storageURI, pipelineName, serviceName, wd string, caches []string, gitFolders []string) *WorkspaceAchiever {
	return &WorkspaceAchiever{
		paths:        caches,
		wd:           wd,
		gitFolders:   gitFolders,
		StorageURI:   storageURI,
		PipelineName: pipelineName,
		ServiceName:  serviceName,
	}
}

type FileInfo struct {
	path string
}

type ByPathName []*FileInfo

func (p ByPathName) Len() int           { return len(p) }
func (p ByPathName) Less(i, j int) bool { return p[i].path < p[j].path }
func (p ByPathName) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (c *WorkspaceAchiever) sortedFiles() []string {
	files := make([]string, len(c.files))

	i := 0
	for file := range c.files {
		files[i] = file
		i++
	}

	sort.Strings(files)
	return files
}

func (c *WorkspaceAchiever) add(path string) (err error) {
	// Always use slashes
	path = filepath.ToSlash(path)

	// Check if file exist
	info, err := os.Lstat(path)
	if err == nil {
		c.files[path] = info
	}
	return
}

func (c *WorkspaceAchiever) process(match string) bool {
	var absolute, relative string
	var err error

	absolute, err = filepath.Abs(match)
	if err == nil {
		// Let's try to find a real relative path to an absolute from working directory
		relative, err = filepath.Rel(c.wd, absolute)
	}
	if err == nil {
		// Process path only if it lives in our build directory
		if !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			err = c.add(relative)
		} else {
			err = errors.New("not supported: outside build directory")
		}
	}
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		// We hide the error that file doesn't exist
		return false
	}

	log.Warningf("%s: %v", match, err)
	return false
}

func (c *WorkspaceAchiever) processPaths(paths []string, verbose bool) {
	for _, path := range paths {
		matches, err := filepath.Glob(filepath.Join(c.wd, path))
		if err != nil {
			log.Warningf("%s: %v", path, err)
			continue
		}

		found := 0

		for _, match := range matches {
			err := filepath.Walk(match, func(path string, info os.FileInfo, err error) error {
				if c.process(path) {
					found++
				}
				return nil
			})
			if err != nil {
				log.Warningf("Walking", match, err)
			}
		}

		if verbose {
			if found == 0 {
				log.Warningf("%s: no matching files", path)
			} else {
				log.Infof("%s: found %d matching files", path, found)
			}
		}
	}
}

func (c *WorkspaceAchiever) enumerate() error {
	c.files = make(map[string]os.FileInfo)

	c.processPaths(c.paths, true)

	for _, folder := range c.gitFolders {
		c.processPaths([]string{filepath.Join(folder, ".git")}, false)
	}

	return nil
}

func (c *WorkspaceAchiever) Achieve(target string) error {
	if err := c.enumerate(); err != nil {
		return err
	}

	f, err := ioutil.TempFile("", "*cached_files.txt")
	if err != nil {
		return err
	}

	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	for _, path := range c.sortedFiles() {
		_, err = f.WriteString(path + "\n")
		if err != nil {
			return fmt.Errorf("failed to create cache file list: %s", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
		return err
	}

	temp, err := ioutil.TempFile("", "*reaper.tar.gz")
	if err != nil {
		log.Errorf("failed to create temporary file: %v", err)
		return err
	}

	_ = temp.Close()

	log.Info("achieving caches ...")
	cmd := exec.Command("tar", "czf", temp.Name(), "--no-recursion", "-C", c.wd, "-T", f.Name())
	cmd.Dir = c.wd
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		log.Errorf("failed to compress %v", err)
		return err
	}

	//if err := helper.Move(temp.Name(), target); err != nil {
	//	log.Errorf("failed to move cache to shared fs %v", err)
	//	return err
	//}

	if store, err := s3.NewS3StorageFromEncryptedURI(c.StorageURI); err == nil {
		forcedPathStyle := false
		if store.Provider == setting.ProviderSourceSystemDefault {
			forcedPathStyle = true
		}
		s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
		if err != nil {
			log.Errorf("Archive s3 create s3 client error: %+v", err)
			return err
		}
		objectKey := store.GetObjectPath(fmt.Sprintf("%s/%s/%s/%s", c.PipelineName, c.ServiceName, "cache", meta.FileName))
		if err = s3client.Upload(store.Bucket, temp.Name(), objectKey); err != nil {
			log.Errorf("Archive s3 upload err:%v", err)
			return err
		}
	}

	return nil

}
