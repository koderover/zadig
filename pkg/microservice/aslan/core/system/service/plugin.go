/*
Copyright 2025 The KodeRover Authors.

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

package service

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func ListPlugins(log *zap.SugaredLogger) ([]*commonmodels.Plugin, error) {
	resp, err := commonrepo.NewPluginColl().List()
	if err != nil {
		log.Errorf("Plugin.List error: %s", err)
		return resp, e.ErrListExternalLink.AddErr(err)
	}
	return resp, nil
}

func DeletePlugin(id string, log *zap.SugaredLogger) error {
	if err := commonrepo.NewPluginColl().Delete(id); err != nil {
		log.Errorf("Plugin.Delete %s error: %s", id, err)
		return e.ErrDeleteExternalLink.AddErr(err)
	}
	return nil
}

// CreatePluginWithFile uploads the given file, analyzes metadata, and persists the plugin.
// NOTE: actual upload destination/path logic is left for you to implement below.
func CreatePluginWithFile(m *commonmodels.Plugin, fileHeader *multipart.FileHeader, file multipart.File, log *zap.SugaredLogger) error {
	// Save to temp file to compute hash/size. You can stream if desired.
	tempDir, err := os.MkdirTemp("", "plugin-upload-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	tempPath := filepath.Join(tempDir, fileHeader.Filename)
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hasher), file)
	_ = out.Close()
	if err != nil {
		return err
	}

	m.FileName = fileHeader.Filename
	m.FileSize = size
	m.FileHash = hex.EncodeToString(hasher.Sum(nil))

	// TODO: Upload tempPath to object storage indicated by m.StorageID and set m.FilePath accordingly

	if err := commonrepo.NewPluginColl().Create(m); err != nil {
		log.Errorf("Plugin.Create error: %s", err)
		return e.ErrCreateExternalLink.AddErr(err)
	}
	return nil
}

// UpdatePluginWithFile handles updating plugin info and replacing the uploaded file (possibly new file name)
// NOTE: actual upload destination/path logic is left for you to implement below.
func UpdatePluginWithFile(id string, m *commonmodels.Plugin, fileHeader *multipart.FileHeader, file multipart.File, log *zap.SugaredLogger) error {
	// Save to temp file to compute hash/size
	tempDir, err := os.MkdirTemp("", "plugin-upload-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	tempPath := filepath.Join(tempDir, fileHeader.Filename)
	out, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(out, hasher), file)
	_ = out.Close()
	if err != nil {
		return err
	}

	m.FileName = fileHeader.Filename
	m.FileSize = size
	m.FileHash = hex.EncodeToString(hasher.Sum(nil))

	// TODO: Upload tempPath to object storage indicated by m.StorageID and set m.FilePath accordingly

	if err := commonrepo.NewPluginColl().Update(id, m); err != nil {
		log.Errorf("Plugin.Update %s error: %s", id, err)
		return e.ErrUpdateExternalLink.AddErr(err)
	}
	return nil
}

// GetPluginFilePath is intentionally left for you to implement the business logic.
// TODO: implement GetPluginFilePath service logic to locate the actual file path and filename for the plugin
func GetPluginFilePath(id string, log *zap.SugaredLogger) (string, string, error) {
	// return absoluteFilePath, fileName, error
	return "", "", nil
}
