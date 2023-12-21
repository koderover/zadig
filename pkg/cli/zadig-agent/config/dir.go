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

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common/types"
	fileutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/file"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
)

const (
	LocalConfig      = ".zadig-agent/agent_config.yaml"
	LocalLogFilePath = ".zadig-agent/log"
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
			return "", fmt.Errorf("failed to create artifacts directory: %v", err)
		}
	}
	return path, nil
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
			return "", fmt.Errorf("failed to create log directory: %v", err)
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
			return "", fmt.Errorf("failed to get user home directory: %v", err)
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
		return "", fmt.Errorf("failed to get agent log directory: %v", err)
	}

	return path, nil
}

func Home() string {
	return viper.GetString(common.Home)
}

func GetJobLogFilePath(workDir string, job types.ZadigJobTask) (string, error) {
	jobLogTmpDir := filepath.Join(workDir, common.JobLogTmpDir)
	if _, err := os.Stat(jobLogTmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(jobLogTmpDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create job log tmp directory: %v", err)
		}
	}

	logDir := filepath.Join(jobLogTmpDir, fmt.Sprintf("%s-%s-%d-%s", job.ProjectName, job.WorkflowName, job.TaskID, job.JobName))
	return logDir, nil
}

func GetJobOutputsTmpDir(workDir string, job types.ZadigJobTask) (string, error) {
	jobOutputsTmpDir := filepath.Join(workDir, common.JobOutputsTmpDir)
	if _, err := os.Stat(jobOutputsTmpDir); os.IsNotExist(err) {
		err := os.MkdirAll(jobOutputsTmpDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create job outputs tmp directory: %v", err)
		}
	}

	dir := filepath.Join(jobOutputsTmpDir, fmt.Sprintf("%s-%s-%d-%s", job.ProjectName, job.WorkflowName, job.TaskID, job.JobName))
	return filepath.Join(dir, common.JobOutputDir), nil
}

func GetJobScriptTmpDir(workDir string, job types.ZadigJobTask) (string, error) {
	jobScriptTmpDir := filepath.Join(workDir, common.JobScriptTmpDir)
	if _, err := os.Stat(jobScriptTmpDir); os.IsNotExist(err) {
		err := os.MkdirAll(jobScriptTmpDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create job script tmp directory: %v", err)
		}
	}

	jobScriptDir := filepath.Join(jobScriptTmpDir, fmt.Sprintf("%s-%s-%d-%s", job.ProjectName, job.WorkflowName, job.TaskID, job.JobName))
	return jobScriptDir, nil
}

func GetCacheDir(workDir string) (string, error) {
	cacheDir := filepath.Join(workDir, common.JobCacheTmpDir)
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		err := os.MkdirAll(cacheDir, os.ModePerm)
		if err != nil {
			return "", fmt.Errorf("failed to create cache directory: %v", err)
		}
	}

	return cacheDir, nil
}

func GetUserShellScriptFilePath(workDir string) string {
	return filepath.Join(workDir, "user_script.sh")
}

func GetUserBatchFileScriptFilePath(workDir string) string {
	return filepath.Join(workDir, "user_script.bat")
}
