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
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	utilfile "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/file"
)

var agentConfig *AgentConfig

func NewAgentConfig() *AgentConfig {
	if agentConfig != nil && agentConfig.Token != "" && agentConfig.ServerURL != "" {
		return agentConfig
	}
	return &AgentConfig{
		Token:         viper.GetString(common.AGENT_TOKEN),
		ServerURL:     viper.GetString(common.ZADIG_SERVER_URL),
		AgentVersion:  viper.GetString(common.AGENT_VERSION),
		InstallTime:   viper.GetInt64(common.AGENT_INSTALL_TIME),
		InstallUser:   viper.GetString(common.AGENT_INSTALL_USER),
		WorkDirectory: viper.GetString(common.AGENT_WORK_DIRECTORY),
	}
}

func SetAgentConfig(config *AgentConfig) {
	if config == nil {
		config = NewAgentConfig()
	}
	agentConfig = config
}

type AgentConfig struct {
	Token             string `yaml:"token"`
	ServerURL         string `yaml:"server_url"`
	VmName            string `yaml:"vm_name"`
	Description       string `yaml:"description"`
	Concurrency       int    `yaml:"concurrency"`
	CacheType         string `yaml:"cache_type"`
	CachePath         string `yaml:"cache_path"`
	AgentVersion      string `yaml:"agent_version"`
	ZadigVersion      string `yaml:"zadig_version"`
	AgentPlatform     string `yaml:"agent_platform"`
	AgentArchitecture string `yaml:"agent_architecture"`
	InstallTime       int64  `yaml:"install_time"`
	InstallUser       string `yaml:"install_user"`
	Status            string `yaml:"status"`
	ErrMsg            string `yaml:"err_msg"`
	ScheduleWorkflow  bool   `yaml:"schedule_workflow"`
	WorkDirectory     string `yaml:"work_directory"`
	BuildGoVersion    string `yaml:"build_go_version"`
	BuildCommit       string `yaml:"build_commit"`
	BuildTime         string `yaml:"build_time"`
	EnableDebug       bool   `yaml:"enable_debug"`
}

func InitConfig() bool {
	path, err := GetAgentConfigFilePath()
	log.Printf("default agent config file path is %s\n", path)
	if err != nil {
		log.Panicf("failed to get agent config file in path: %v", err)
	}

	exists, err := utilfile.FileExists(path)
	if err != nil {
		log.Panicf("failed to check agent config file: %v", err)
	}

	// load config file for agent
	if exists {
		config := &AgentConfig{}
		yamlFile, err := ioutil.ReadFile(path)
		if err != nil {
			log.Panicf("failed to read agent config file: %v", err)
		}

		err = yaml.Unmarshal(yamlFile, config)
		if err != nil {
			log.Panicf("failed to unmarshal agent config file: %v", err)
		}
		if config.Token != "" && config.ServerURL != "" {
			agentConfig = config
			return true
		}
		err = os.Remove(path)
		if err != nil {
			log.Panicf("failed to remove agent config file: %v", err)
		}
	}
	return false
}

func GetZadigVersion() string {
	return agentConfig.ZadigVersion
}

func SetZadigVersion(version string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}
	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.ZadigVersion = version
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.ZadigVersion = version
}

func SetAgentStatus(status string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}
	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.Status = status
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.Status = status
}

func GetScheduleWorkflow() bool {
	return agentConfig.ScheduleWorkflow
}

func SetScheduleWorkflow(scheduleWorkflow bool) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}
	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.ScheduleWorkflow = scheduleWorkflow
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.ScheduleWorkflow = scheduleWorkflow
}

func SetAgentToken(token string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}

	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.Token = token
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.Token = token
}

func SetServerURL(serverURL string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}

	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.ServerURL = serverURL
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.ServerURL = serverURL
}

func SetAgentErrMsg(errMsg string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}

	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.ErrMsg = errMsg
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.ErrMsg = errMsg
}

func SetAgentWorkDirectory(workDirectory string) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}

	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.WorkDirectory = workDirectory
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}

	agentConfig.WorkDirectory = workDirectory
}

func GetAgentVmName() string {
	return agentConfig.VmName
}

func GetAgentToken() string {
	return agentConfig.Token
}

func GetServerURL() string {
	return agentConfig.ServerURL
}

func GetAgentStatus() string {
	return agentConfig.Status
}

func GetConcurrency() int {
	if agentConfig.Concurrency > 0 {
		return agentConfig.Concurrency
	}
	return common.DefaultAgentConcurrency
}

func SetConcurrency(concurrency int) {
	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}
	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	config := &AgentConfig{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}
	config.Concurrency = concurrency
	err = UpdateAgentConfigFile(config, path)
	if err != nil {
		log.Panicf("failed to update agent config file: %v", err)
	}
	agentConfig.Concurrency = concurrency
}

func GetWorkDirectory() string {
	return agentConfig.WorkDirectory
}

func GetEnableDebug() bool {
	return agentConfig.EnableDebug
}

func GetAgentConfig() (*AgentConfig, error) {
	if agentConfig == nil {
		path, err := GetAgentConfigFilePath()
		if err != nil {
			log.Panicf("failed to get agent config file path: %v", err)
		}

		exists, err := utilfile.FileExists(path)
		if err != nil {
			log.Panicf("failed to check agent config file: %v", err)
		}

		// load config file for agent
		if exists {
			config := &AgentConfig{}
			yamlFile, err := ioutil.ReadFile(path)
			if err != nil {
				log.Panicf("failed to read agent config file: %v", err)
			}

			err = yaml.Unmarshal(yamlFile, config)
			if err != nil {
				log.Panicf("failed to unmarshal agent config file: %v", err)
			}

			agentConfig = config
			return config, nil
		} else {
			return nil, fmt.Errorf("agent config file not found")
		}
	}
	return agentConfig, nil
}

func UpdateAgentConfigFile(config *AgentConfig, path string) error {
	yamlConfig, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal agent config file: %v", err)
	}

	err = ioutil.WriteFile(path, yamlConfig, 0644)
	if err != nil {
		return fmt.Errorf("failed to write agent config file: %v", err)
	}
	return nil
}

func BatchUpdateAgentConfig(config *AgentConfig) error {
	if config == nil {
		return nil
	}

	if agentConfig == nil {
		log.Panicf("agent config is nil")
	}

	path, err := GetAgentConfigFilePathWithCheck()
	if err != nil {
		log.Panicf("failed to get agent config file path: %v", err)
	}

	// update config file
	oldConfig := new(AgentConfig)
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Panicf("failed to read agent config file: %v", err)
	}

	err = yaml.Unmarshal(yamlFile, oldConfig)
	if err != nil {
		log.Panicf("failed to unmarshal agent config file: %v", err)
	}

	// if token is empty, indicate that the agent is not registered
	if oldConfig.Token == "" {
		oldConfig.Token = agentConfig.Token
		oldConfig.ServerURL = agentConfig.ServerURL
		oldConfig.AgentVersion = agentConfig.AgentVersion
		oldConfig.InstallTime = agentConfig.InstallTime
		oldConfig.InstallUser = agentConfig.InstallUser
		oldConfig.WorkDirectory = agentConfig.WorkDirectory
	}

	if config.ServerURL != "" {
		oldConfig.ServerURL = config.ServerURL
	}

	if config.VmName != "" {
		oldConfig.VmName = config.VmName
	}

	if config.Description != "" {
		oldConfig.Description = config.Description
	}

	oldConfig.ScheduleWorkflow = config.ScheduleWorkflow

	if config.WorkDirectory != "" {
		oldConfig.WorkDirectory = config.WorkDirectory
	}

	if config.Concurrency > 0 {
		oldConfig.Concurrency = config.Concurrency
	}

	if config.CacheType != "" {
		oldConfig.CacheType = config.CacheType
	}

	if config.ZadigVersion != "" {
		oldConfig.ZadigVersion = config.ZadigVersion
	}

	oldConfig.AgentVersion = BuildAgentVersion
	oldConfig.BuildCommit = BuildCommit
	oldConfig.BuildGoVersion = BuildGoVersion
	oldConfig.BuildTime = BuildTime

	err = UpdateAgentConfigFile(oldConfig, path)
	if err != nil {
		return fmt.Errorf("failed to update agent config file: %v", err)
	}
	agentConfig = oldConfig
	return nil
}
