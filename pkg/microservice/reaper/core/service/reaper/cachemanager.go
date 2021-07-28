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

package reaper

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/mholt/archiver.v3"

	"github.com/koderover/zadig/pkg/microservice/reaper/core/service/meta"
	"github.com/koderover/zadig/pkg/microservice/reaper/internal/s3"
	"github.com/koderover/zadig/pkg/setting"
	"github.com/koderover/zadig/pkg/tool/log"
	s3tool "github.com/koderover/zadig/pkg/tool/s3"
	"github.com/koderover/zadig/pkg/util"
)

func getAchiver() *archiver.TarGz {
	return &archiver.TarGz{
		Tar: &archiver.Tar{
			OverwriteExisting:      true,
			MkdirAll:               true,
			ImplicitTopLevelFolder: false,
		},
	}
}

// CacheManager manages the caches
type CacheManager interface {
	// Archive compress the source folder to dest file
	Archive(source, dest string) error
	// Unarchive decompress the source file to dest folder
	Unarchive(source, dest string) error
}

// GoCacheManager is deprecated
type GoCacheManager struct{}

func NewGoCacheManager() *GoCacheManager {
	return &GoCacheManager{}
}

func (gcm *GoCacheManager) Archive(source, dest string) error {
	var sources []string
	files, err := ioutil.ReadDir(source)
	if err != nil {
		return err
	}
	for _, f := range files {
		sources = append(sources, filepath.Join(source, f.Name()))
	}

	return getAchiver().Archive(sources, dest)
}

func (gcm *GoCacheManager) Unarchive(source, dest string) error {
	return getAchiver().Unarchive(source, dest)
}

type TarCacheManager struct {
	StorageURI   string
	PipelineName string
	ServiceName  string
}

func NewTarCacheManager(storageURI, pipelineName, serviceName string) *TarCacheManager {
	return &TarCacheManager{
		StorageURI:   storageURI,
		PipelineName: pipelineName,
		ServiceName:  serviceName,
	}
}

func (gcm *TarCacheManager) Archive(source, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}

	temp, err := ioutil.TempFile("", "*reaper.tar.gz")
	if err != nil {
		log.Errorf("failed to create temp file %v", err)
		return err
	}

	_ = temp.Close()

	cmd := exec.Command("tar", "czf", temp.Name(), "-C", source, ".")
	cmd.Dir = source
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err = cmd.Run(); err != nil {
		log.Errorf("failed to compress cache %v", err)
		return err
	}

	//if err = helper.Move(temp.Name(), dest); err != nil {
	//	log.Errorf("failed to upload cache to shared fs %v", err)
	//	return err
	//}

	if store, err := gcm.getS3Storage(); err == nil {
		forcedPathStyle := false
		if store.Provider == setting.ProviderSourceSystemDefault {
			forcedPathStyle = true
		}
		s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
		if err != nil {
			log.Errorf("Archive s3 create s3 client error: %+v", err)
			return err
		}
		objectKey := store.GetObjectPath(meta.FileName)
		if err = s3client.Upload(store.Bucket, temp.Name(), objectKey); err != nil {
			log.Errorf("Archive s3 upload err:%v", err)
			return err
		}
	}

	return nil
}

func (gcm *TarCacheManager) Unarchive(source, dest string) error {
	if store, err := gcm.getS3Storage(); err == nil {
		forcedPathStyle := false
		if store.Provider == setting.ProviderSourceSystemDefault {
			forcedPathStyle = true
		}
		s3client, err := s3tool.NewClient(store.Endpoint, store.Ak, store.Sk, store.Insecure, forcedPathStyle)
		if err != nil {
			return err
		}
		files, _ := s3client.ListFiles(store.Bucket, store.GetObjectPath(meta.FileName), false)
		if len(files) > 0 {
			if sourceFilename, err := util.GenerateTmpFile(); err == nil {
				defer func() {
					_ = os.Remove(sourceFilename)
				}()
				objectKey := store.GetObjectPath(files[0])
				err = s3client.Download(store.Bucket, objectKey, sourceFilename)
				if err != nil {
					return err
				}
				if err = os.MkdirAll(dest, os.ModePerm); err != nil {
					return err
				}
				out := bytes.NewBufferString("")
				cmd := exec.Command("tar", "xzf", sourceFilename, "-C", dest)
				cmd.Stderr = out
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("%s %v", out.String(), err)
				}

			}
		} else {
			return fmt.Errorf("cache not found")
		}
	} else {
		return err
	}
	return nil
}

func (gcm *TarCacheManager) getS3Storage() (*s3.S3, error) {
	var err error
	var store *s3.S3
	if store, err = s3.NewS3StorageFromEncryptedURI(gcm.StorageURI); err != nil {
		log.Errorf("Archive failed to create s3 storage %s", gcm.StorageURI)
		return nil, err
	}
	if store.Subfolder != "" {
		store.Subfolder = fmt.Sprintf("%s/%s/%s/%s", store.Subfolder, gcm.PipelineName, gcm.ServiceName, "cache")
	} else {
		store.Subfolder = fmt.Sprintf("%s/%s/%s", gcm.PipelineName, gcm.ServiceName, "cache")
	}
	return store, nil
}
