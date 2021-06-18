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

package testing

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/koderover/zadig/pkg/tool/log"
	fsutil "github.com/koderover/zadig/pkg/util/fs"
)

func init() {
	log.Init(&log.Config{Level: "debug"})

	// TODO: LOU: clean the path when test is done
	tmpRoot, err := os.MkdirTemp("", "")
	if err != nil {
		log.DPanic(err)
	}

	// prepare the aes key file
	createFakeKeyFile(tmpRoot)

	fsutil.Chroot(tmpRoot)
}

func createFakeKeyFile(root string) {
	if err := os.MkdirAll(filepath.Join(root, "etc", "encryption"), fs.ModePerm); err != nil {
		log.DPanic(err)
	}

	f, err := os.Create(filepath.Join(root, "etc", "encryption", "aes"))
	if err != nil {
		log.DPanic(err)
	}

	_, err = f.WriteString("9F11B4E503C7F2B577E5F9366BDDAB64")
	if err != nil {
		log.DPanic(err)
	}

	err = f.Close()
	if err != nil {
		log.DPanic(err)
	}
}
