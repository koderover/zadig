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

package helper

import (
	"bufio"
	"io"
	"os"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/text/encoding/simplifiedchinese"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	agentutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/file"
	"github.com/koderover/zadig/v2/pkg/util"
)

const (
	secretEnvMask = "********"
)

func IsDirEmpty(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return true
	}
	defer f.Close()

	_, err = f.Readdir(1)
	return err == io.EOF
}

func SetCmdsWorkDir(dir string, cmds []*common.Command) {
	for _, c := range cmds {
		c.Cmd.Dir = dir
	}
}

func HandleCmdOutput(pipe io.ReadCloser, needPersistentLog bool, logFile string, secretEnvs []string, logger *zap.SugaredLogger) {
	reader := bufio.NewReader(pipe)

	for {
		lineBytes, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}

			logger.Errorf("Failed to read log when processing cmd output: %s", err)
			break
		}

		if runtime.GOOS == "windows" {
			decodeBytes, err := simplifiedchinese.GB18030.NewDecoder().Bytes(lineBytes)
			if err != nil {
				logger.Errorf("failed to decode to GB18030, source: %s, err: %v", lineBytes, err)
			} else {
				tmpStr := string(decodeBytes)
				if strings.HasSuffix(tmpStr, "\r\n") {
					tmpStr = strings.TrimSuffix(tmpStr, "\r\n") + "\n"
					lineBytes = []byte(tmpStr)
				}
			}
		}

		if needPersistentLog {
			err := agentutil.WriteFile(logFile, []byte(util.MaskSecretEnvs(string(lineBytes), secretEnvs)), 0700)
			if err != nil {
				logger.Warnf("Failed to write file when processing cmd output: %s", err)
			}
		}
	}
}
