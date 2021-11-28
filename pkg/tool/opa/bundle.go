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

package opa

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/27149chen/afero"

	"github.com/koderover/zadig/pkg/tool/log"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

const (
	manifestPath = ".manifest"
)

type Manifest struct {
	Revision string   `json:"revision"`
	Roots    []string `json:"roots"`
}

type DataSpec struct {
	Data interface{}
	Path string
}

type serializedDataSpec struct {
	Content []byte
	Path    string
}

type Bundle struct {
	Data       []*DataSpec
	Roots      []string
	hash       string
	serialized []*serializedDataSpec
}

func (b *Bundle) Rehash() (string, error) {
	if !b.validate() {
		return "", fmt.Errorf("validation failed, data or roots is missing")
	}

	var allContents []byte
	var err error
	b.serialized = nil

	for _, file := range b.Data {
		var content []byte
		switch c := file.Data.(type) {
		case []byte:
			content = c
		default:
			content, err = json.MarshalIndent(c, "", "    ")
			if err != nil {
				log.Errorf("Failed to marshal file %s, err: %s", file.Path, err)
				return "", err
			}
		}

		b.serialized = append(b.serialized, &serializedDataSpec{
			Content: content,
			Path:    file.Path,
		})
		allContents = append(allContents, content...)
	}

	h := sha256.New()
	h.Write(allContents)
	b.hash = fmt.Sprintf("%x", h.Sum(nil))

	return b.hash, nil
}

func (b *Bundle) Hash() (string, error) {
	if b.hash != "" {
		return b.hash, nil
	}

	return b.Rehash()
}

func (b *Bundle) Save(parentPath string) error {
	mf, err := b.serializeOPAManifest()
	if err != nil {
		log.Errorf("Failed to serialize OPA manifest, err: %s", err)
		return err
	}
	b.serialized = append(b.serialized, mf)

	cacheFS := afero.NewMemMapFs()
	for _, file := range b.serialized {
		err = cacheFS.MkdirAll(filepath.Dir(file.Path), 0755)
		if err != nil {
			log.Errorf("Failed to create path %s, err: %s", filepath.Dir(file.Path), err)
			return err
		}
		err = afero.WriteFile(cacheFS, file.Path, file.Content, 0644)
		if err != nil {
			log.Errorf("Failed to write file %s, err: %s", file.Path, err)
			return err
		}
	}

	tarball := "bundle.tar.gz"
	path := filepath.Join(parentPath, tarball)
	if err = fsutil.Tar(afero.NewIOFS(cacheFS), path); err != nil {
		log.Errorf("Failed to archive tarball %s, err: %s", path, err)
		return err
	}

	return nil
}

func (b *Bundle) generateOPAManifest() (*Manifest, error) {
	revision, err := b.Hash()
	if err != nil {
		log.Errorf("Failed to get hash, err: %s", err)
		return nil, err
	}

	return &Manifest{
		Revision: revision,
		Roots:    b.Roots,
	}, nil
}

func (b *Bundle) serializeOPAManifest() (*serializedDataSpec, error) {
	mf, err := b.generateOPAManifest()
	if err != nil {
		return nil, err
	}

	content, err := json.MarshalIndent(mf, "", "    ")
	if err != nil {
		return nil, err
	}

	return &serializedDataSpec{
		Content: content,
		Path:    manifestPath,
	}, nil
}

func (b *Bundle) validate() bool {
	return len(b.Data) > 0 && len(b.Roots) > 0
}
