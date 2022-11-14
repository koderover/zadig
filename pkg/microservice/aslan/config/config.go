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

	configbase "github.com/koderover/zadig/pkg/config"
	"github.com/koderover/zadig/pkg/setting"
)

func DefaultIngressClass() string {
	return viper.GetString(setting.ENVDefaultIngressClass)
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

func PodName() string {
	return viper.GetString(setting.ENVPodName)
}

func Namespace() string {
	return viper.GetString(setting.ENVNamespace)
}

func LogLevel() int {
	return viper.GetInt(setting.ENVLogLevel)
}

func CollieAPIAddress() string {
	return configbase.CollieServiceAddress()
}

func MongoURI() string {
	return configbase.MongoURI()
}

func MongoDatabase() string {
	return configbase.MongoDatabase()
}

func NsqLookupAddrs() []string {
	return strings.Split(viper.GetString(setting.ENVNsqLookupAddrs), ",")
}

func HubServerAddress() string {
	return configbase.HubServerServiceAddress()
}

func HubAgentImage() string {
	return viper.GetString(setting.ENVHubAgentImage)
}

func ResourceServerImage() string {
	return viper.GetString(setting.ENVResourceServerImage)
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

func Home() string {
	return viper.GetString(setting.Home)
}

func EnableGitCheck() bool {
	return true
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

func PackagerImage() string {
	return viper.GetString(setting.EnvPackagerImage)
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

func WebHookURL() string {
	return fmt.Sprintf("%s/api/aslan/webhook", configbase.SystemAddress())
}

func ObjectStorageServicePath(project, service string) string {
	return configbase.ObjectStorageServicePath(project, service)
}

func LocalServicePath(project, service string) string {
	return configbase.LocalServicePathWithRevision(project, service, "latest")
}

func LocalServicePathWithRevision(project, service string, revision int64) string {
	return configbase.LocalServicePathWithRevision(project, service, fmt.Sprintf("%d", revision))
}

func LocalDeliveryChartPathWithRevision(project, service string, revision int64) string {
	return configbase.LocalServicePathWithRevision(project, service, fmt.Sprintf("delivery/%d", revision))
}

func ServiceNameWithRevision(serviceName string, revision int64) string {
	return fmt.Sprintf("%s-%d", serviceName, revision)
}

func ServiceAccountNameForUser(userID string) string {
	return fmt.Sprintf("%s-sa", userID)
}

func DindImage() string {
	return viper.GetString(setting.DindImage)
}

func MysqlDexDB() string {
	return viper.GetString(setting.ENVMysqlDexDB)
}

func Features() string {
	return viper.GetString(setting.FeatureFlag)
}

func MysqlUserDB() string {
	return viper.GetString(setting.ENVMysqlUserDB)
}
