/*
Copyright 2023 The KodeRover Authors.

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

package job

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/agent/step/helper"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
)

var cache = &cacheLock{
	cm: make(map[string]struct{}),
	mu: sync.Mutex{},
}

type cacheLock struct {
	mu sync.Mutex
	cm map[string]struct{}
}

func (l *cacheLock) Lock(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cm == nil {
		l.cm = make(map[string]struct{})
	}

	for {
		if _, ok := l.cm[key]; ok {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		break
	}
	l.cm[key] = struct{}{}
}

func (l *cacheLock) Unlock(key string) {
	delete(l.cm, key)
}

func ReadCache(job types.ZadigJobTask, dest, cacheDir string, logger *zap.SugaredLogger) {
	// lock the cache path to avoid other job write or read this path at the same time
	key := fmt.Sprintf("%s-%s-%s", job.ProjectName, job.WorkflowName, job.JobName)
	cache.Lock(key)
	defer cache.Unlock(key)

	cachePath := filepath.Join(cacheDir, job.ProjectName, job.WorkflowName, job.JobName)
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		err := os.MkdirAll(cachePath, os.ModePerm)
		if err != nil {
			log.Errorf("failed to create cache directory %s, error: %v", cachePath, err)
		}
		return
	} else {
		err = copyCmd(cachePath, dest, logger)
		if err != nil {
			log.Errorf("failed to copy cache directory %s to %s, error: %v", cachePath, dest, err)
		}
	}
}

func WriteCache(job types.ZadigJobTask, src string, cachePath string, logger *zap.SugaredLogger) {
	// lock the cache path to avoid other job write or read this path at the same time
	key := fmt.Sprintf("%s-%s-%s", job.ProjectName, job.WorkflowName, job.JobName)
	cache.Lock(key)
	defer cache.Unlock(key)

	if _, err := os.Stat(cachePath); err == os.ErrNotExist {
		err := os.MkdirAll(cachePath, os.ModePerm)
		if err != nil {
			log.Errorf("failed to create cache directory %s, error: %v", cachePath, err)
		}
	}

	err := copyCmd(src, cachePath, logger)
	if err != nil {
		log.Errorf("failed to copy cache directory %s to %s, error: %v", src, cachePath, err)
	}
}

func copyCmd(src, dest string, logger *zap.SugaredLogger) error {
	cmd := exec.Command("cp", "-f", "-r", src, dest)
	var wg sync.WaitGroup

	cmdStdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	// write script output to log file
	wg.Add(1)
	go func() {
		defer wg.Done()

		helper.HandleCmdOutput(cmdStdoutReader, true, log.GetLoggerFile(), nil, logger)
	}()

	cmdStdErrReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		helper.HandleCmdOutput(cmdStdErrReader, true, log.GetLoggerFile(), nil, logger)
	}()

	if err := cmd.Start(); err != nil {
		return err
	}

	wg.Wait()

	return cmd.Wait()
}
