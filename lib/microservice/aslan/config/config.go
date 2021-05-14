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

package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	_ "github.com/koderover/zadig/lib/config"
	"github.com/koderover/zadig/lib/setting"
	mongotool "github.com/koderover/zadig/lib/tool/mongo"
)

func init() {
	mongoURI := viper.GetString(setting.ENVMongoDBConnectionString)
	db, uri, err := mongotool.ExtractDatabaseName(mongoURI)
	if err != nil {
		panic(fmt.Errorf("wrong mongo address, err: %s", err))
	}

	viper.Set(setting.ENVMongoDBConnectionString, uri)
	if db != "" {
		viper.Set(setting.ENVAslanDBName, db)
	}
}

func DefaultIngressClass() string {
	ingressClass := viper.GetString(setting.ENVDefaultIngressClass)
	if ingressClass == "system" {
		return ""
	} else if ingressClass == "" {
		return setting.DefaultIngressClass
	}

	return ingressClass
}

// 服务默认等待启动时间，默认5分钟
func ServiceStartTimeout() int {
	serviceStartTimeout := viper.GetString(setting.ENVServiceStartTimeout)
	if serviceStartTimeout == "" {
		return 300
	}

	serviceStartTimeoutValue, err := strconv.ParseInt(serviceStartTimeout, 10, 32)
	if err != nil || serviceStartTimeoutValue < 60 {
		panic(errors.New("SERVICE_START_TIMEOUT is not int or less than 60"))
	}

	return int(serviceStartTimeoutValue)
}

// 环境默认回收天数，默认为0
func DefaultRecycleDay() int {
	defaultRecycleDay := viper.GetString(setting.ENVDefaultEnvRecycleDay)
	if defaultRecycleDay == "" {
		return 0
	}

	defaultRecycleDayValue, err := strconv.ParseInt(defaultRecycleDay, 10, 32)
	if err != nil {
		panic(errors.New("DEFAULT_ENV_RECYCLE_DAY is not int"))
	}

	return int(defaultRecycleDayValue)
}

func HookSecret() string {
	hs := viper.GetString(setting.ENVHookSecret)
	if hs == setting.Unset {
		return ""
	}

	return hs
}

func PodName() string {
	return viper.GetString(setting.ENVPodName)
}

func Namespace() string {
	return viper.GetString(setting.ENVNamespace)
}

func LogLevel() int {
	return viper.GetInt(setting.ENVLogLevel)
}

func PoetryAPIServer() string {
	return viper.GetString(setting.ENVPoetryAPIServer)
}

func PoetryAPIRootKey() string {
	return viper.GetString(setting.ENVPoetryAPIRootKey)
}

func CollieAPIAddress() string {
	return viper.GetString(setting.ENVCollieAPIAddress)
}

func MongoURI() string {
	return viper.GetString(setting.ENVMongoDBConnectionString)
}

func MongoDatabase() string {
	return viper.GetString(setting.ENVAslanDBName)
}

func NsqLookupAddrs() []string {
	return strings.Split(viper.GetString(setting.ENVNsqLookupAddrs), ",")
}

func AslanURL() string {
	return viper.GetString(setting.ENVAslanURL)
}

func HubServerAddress() string {
	return viper.GetString(setting.ENVHubServerAddr)
}

func KodespaceVersion() string {
	return viper.GetString(setting.ENVKodespaceVersion)
}

// CleanIgnoredList is a list which will be ignored during environment cleanup.
func CleanSkippedList() []string {
	return strings.Split(viper.GetString(setting.CleanSkippedList), ",")
}

// FIXME FIXME FIXME FIXME delete constant
func S3StoragePath() string {
	return "/var/lib/workspace"
}

func EnableGitCheck() bool {
	return viper.GetString(setting.EnableGitCheck) == "true"
}

func S3StorageAK() string {
	return viper.GetString(setting.ENVS3StorageAK)
}

func S3StorageSK() string {
	return viper.GetString(setting.ENVS3StorageSK)
}

func S3StorageBucket() string {
	return viper.GetString(setting.ENVS3StorageBucket)
}

func S3StorageEndpoint() string {
	return viper.GetString(setting.ENVS3StorageEndpoint)
}

func S3StorageProtocol() string {
	return viper.GetString(setting.ENVS3StorageProtocol)
}

func AslanAPIBase() string {
	return viper.GetString(setting.ENVAslanAPIBase)
}

func SetProxy(HTTPSAddr, HTTPAddr, Socks5Addr string) {
	viper.Set(setting.ProxyHTTPSAddr, HTTPSAddr)
	viper.Set(setting.ProxyHTTPAddr, HTTPAddr)
	viper.Set(setting.ProxySocks5Addr, Socks5Addr)
}

func ProxyHTTPSAddr() string {
	return viper.GetString(setting.ProxyHTTPSAddr)
}

func ProxyHTTPAddr() string {
	return viper.GetString(setting.ProxyHTTPAddr)
}

func KubeServerAddr() string {
	return viper.GetString(setting.ENVKubeServerAddr)
}

func RegistryAddress() string {
	return viper.GetString(setting.ENVAslanRegAddress)
}

func RegistryAccessKey() string {
	return viper.GetString(setting.ENVAslanRegAccessKey)
}

func RegistrySecretKey() string {
	return viper.GetString(setting.ENVAslanRegSecretKey)
}

func RegistryNamespace() string {
	return viper.GetString(setting.ENVAslanRegNamespace)
}

func ENVAslanURL() string {
	return viper.GetString(setting.ENVAslanURL)
}

func ENVWarpdriveService() string {
	return viper.GetString(setting.ENVWarpdriveService)
}

func GithubSSHKey() string {
	return viper.GetString(setting.ENVGithubSSHKey)
}

func GithubKnownHost() string {
	return viper.GetString(setting.ENVGithubKnownHost)
}

func ReaperImage() string {
	return viper.GetString(setting.ENVReaperImage)
}

func ReaperBinaryFile() string {
	return viper.GetString(setting.ENVReaperBinaryFile)
}

func PredatorImage() string {
	return viper.GetString(setting.ENVPredatorImage)
}

func DockerHosts() []string {
	return strings.Split(viper.GetString(setting.ENVDockerHosts), ",")
}

func UseClassicBuild() bool {
	return viper.GetString(setting.ENVUseClassicBuild) == "true"
}

func CustomDNSNotSupported() bool {
	return !(viper.GetString(setting.ENVCustomDNSNotSupported) == "true")
}

func OldEnvSupported() bool {
	return viper.GetString(setting.ENVOldEnvSupported) == "true"
}

func ProxySocks5Addr() string {
	return viper.GetString(setting.ProxySocks5Addr)
}

func JenkinsImage() string {
	return viper.GetString(setting.JenkinsBuildImage)
}

func SonarAddr() string {
	return viper.GetString(setting.EnvSonarAddr)
}
