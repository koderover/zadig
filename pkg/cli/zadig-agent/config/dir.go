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

package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/cli/zadig-agent/internal/common"
	fileutil "github.com/koderover/zadig/pkg/cli/zadig-agent/util/file"
	osutil "github.com/koderover/zadig/pkg/cli/zadig-agent/util/os"
)

const (
	LocalConfig      = ".zadig-agent/agent_config.yaml"
	LocalLogFilePath = ".zadig-agent/log"
	JobLogFilePath   = "/log"
	JobCacheDir      = "/.cache"
	JobArtifactsDir  = "/artifacts"
)

func GetActiveWorkDirectory() string {
	directory := GetWorkDirectory()
	if directory == "" {
		defaultDir, err := GetAgentWorkDir("")
		if err != nil {
			log.Panicf("failed to get agent working directory: %v", err)
		}
		directory = defaultDir
	}
	return directory
}

func GetJobLogFilePath(workDir, job string) (string, error) {
	path := fmt.Sprintf("%s%s", workDir, JobLogFilePath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create log directory: %s", err)
		}
	}

	filePath := filepath.Join(path, fmt.Sprintf("%s.log", job))
	// remove old log file if exists
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			return "", fmt.Errorf("failed to remove log file: %s", err)
		}
	}
	// create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create log file: %s", err)
	}
	defer file.Close()

	return filePath, nil
}

func CreateFileIfNotExist(path string) error {
	// check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// create file
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()
	}

	return nil
}

func GetArtifactsDir(workDir string) (string, error) {
	path := filepath.Join(workDir, JobArtifactsDir)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create artifacts directory: %s", err)
		}
	}
	return path, nil
}

func GetUserScriptFilePath(workDir string) string {
	return filepath.Join(workDir, "user_script.sh")

}

func GetAgentConfigFilePath() (string, error) {
	homeDir, err := osutil.GetUserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(homeDir, LocalConfig)
	return path, nil
}

func GetAgentLogFilePath() (string, error) {
	homeDir, err := osutil.GetUserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(homeDir, LocalLogFilePath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create log directory: %s", err)
		}
	}

	return path, nil
}

func GetStopFilePath() (string, error) {
	homeDir, err := osutil.GetUserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(homeDir, ".zadig-agent/.stop")
	return path, nil
}

func GetAgentWorkDir(dir string) (string, error) {
	if dir == "" {
		home, err := osutil.GetUserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %s", err)
		}
		dir = home
	}
	return path.Join(dir, "zadig-agent"), nil
}

func GetAgentConfigFilePathWithCheck() (string, error) {
	path, err := GetAgentConfigFilePath()
	if err != nil {
		return "", err
	}

	exist, err := fileutil.FileExists(path)
	if err != nil {
		return "", err
	}
	if exist {
		return path, nil
	} else {
		return "", fmt.Errorf("agent config file not found")
	}
}

func GetAgentLogPath() (string, error) {
	path, err := GetAgentLogFilePath()
	if err != nil {
		return "", fmt.Errorf("failed to get agent log directory: %s", err)
	}

	return path, nil
}

func Home() string {
	return viper.GetString(common.Home)
}
