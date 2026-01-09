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

	configbase "github.com/koderover/zadig/v2/pkg/config"
	"github.com/koderover/zadig/v2/pkg/setting"
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

func CollieAPIAddress() string {
	return configbase.CollieServiceAddress()
}

func MongoURI() string {
	return configbase.MongoURI()
}

func MongoDatabase() string {
	return configbase.MongoDatabase()
}

func HubAgentImage() string {
	return viper.GetString(setting.ENVHubAgentImage)
}

func ExecutorImage() string {
	return viper.GetString(setting.ENVExecutorImage)
}

func ExecutorLogLevel() string {
	logLevel := viper.GetString(setting.ENVExecutorLogLevel)
	if len(logLevel) == 0 {
		return "info"
	}
	return logLevel
}

func KodespaceVersion() string {
	return viper.GetString(setting.ENVKodespaceVersion)
}

func EnableTransaction() bool {
	return viper.GetString(setting.ENVEnableTransaction) == "true"
}

// CleanIgnoredList is a list which will be ignored during environment cleanup.
func CleanSkippedList() []string {
	return strings.Split(viper.GetString(setting.CleanSkippedList), ",")
}

// S3StoragePath returns a local path used to store downloaded code and other files
func S3StoragePath() string {
	return "/app/data/workspace"
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

func BuildBaseImage() string {
	return viper.GetString(setting.ENVBuildBaseImage)
}

func BuildKitImage() string {
	return viper.GetString(setting.ENVBuildKitImage)
}

func ProxySocks5Addr() string {
	return viper.GetString(setting.ProxySocks5Addr)
}

func ObjectStorageServicePath(project, service string, production bool) string {
	if production {
		return ObjectStorageProductionServicePath(project, service)
	} else {
		return ObjectStorageTestServicePath(project, service)
	}
}

func ObjectStorageTestServicePath(project, service string) string {
	return configbase.ObjectStorageServicePath(project, service)
}

func ObjectStorageProductionServicePath(project, service string) string {
	return configbase.ObjectStorageProductionServicePath(project, service)
}

func LocalServicePath(project, service string, production bool) string {
	if production {
		return LocalProductionServicePath(project, service)
	} else {
		return LocalTestServicePath(project, service)
	}
}

// LocalTestServicePath returns the path of the normal service with the latest version
func LocalTestServicePath(project, service string) string {
	return configbase.LocalTestServicePathWithRevision(project, service, "latest")
}

// LocalProductionServicePath returns the path of the production service with the latest version
func LocalProductionServicePath(project, service string) string {
	return configbase.LocalProductionServicePathWithRevision(project, service, "latest")
}

func LocalServicePathWithRevision(project, service string, version string, production bool) string {
	if production {
		return LocalProductionServicePathWithRevision(project, service, version)
	} else {
		return LocalTestServicePathWithRevision(project, service, version)
	}
}

// LocalTestServicePathWithRevision returns the path of the normal service with the specific revision
func LocalTestServicePathWithRevision(project, service string, revision string) string {
	return configbase.LocalTestServicePathWithRevision(project, service, revision)
}

// LocalProductionServicePathWithRevision returns the path of the normal service with the specific revision
func LocalProductionServicePathWithRevision(project, service string, version string) string {
	return configbase.LocalProductionServicePathWithRevision(project, service, version)
}

// LocalDeliveryChartPathWithRevision returns the path of the normal service with the specific revision
func LocalDeliveryChartPathWithRevision(project, service string, revision int64) string {
	return configbase.LocalTestServicePathWithRevision(project, service, fmt.Sprintf("delivery/%d", revision))
}

func LocalProductionDeliveryChartPathWithRevision(project, service string, revision int64) string {
	return configbase.LocalProductionServicePathWithRevision(project, service, fmt.Sprintf("delivery/%d", revision))
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

func Features() string {
	return viper.GetString(setting.FeatureFlag)
}
