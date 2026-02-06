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
	"fmt"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/koderover/zadig/v2/pkg/setting"
)

// SystemAddress is the fully qualified domain name of the system, or an IP Address.
// Port and protocol are required if necessary.
// for example: foo.bar.com, https://for.bar.com, http://1.2.3.4:5678
func SystemAddress() string {
	return viper.GetString(setting.ENVSystemAddress)
}

func ImagePullPolicy() string {
	return viper.GetString(setting.ENVImagePullPolicy)
}

func ChartVersion() string {
	return viper.GetString(setting.ENVChartVersion)
}

func Mode() string {
	mode := viper.GetString(setting.ENVMode)
	if mode == "" {
		return setting.DebugMode
	}

	return mode
}

// LogLevel returns the configured log level, returns info if unset
func LogLevel() string {
	logLevel := viper.GetString(setting.ENVLogLevel)
	if len(logLevel) == 0 {
		return "info"
	}
	return logLevel
}

func SendLogToFile() bool {
	return true
}

func LogPath() string {
	return fmt.Sprintf("/var/log/%s/", setting.ProductName)
}

func LogName() string {
	return "product.log"
}

func RequestLogName() string {
	return "request.log"
}

func LogFile() string {
	return LogPath() + LogName()
}

func RequestLogFile() string {
	return LogPath() + RequestLogName()
}

func GetServiceByCode(code int) *setting.ServiceInfo {
	return setting.Services[code]
}

func AslanServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Aslan)
}

func UserServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.User)
}

func SecretKey() string {
	return viper.GetString(setting.ENVSecretKey)
}

func AslanServiceAddress() string {
	s := AslanServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func UserServiceAddress() string {
	s := UserServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func HubServerServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.HubServer)
}

func HubServerServiceAddress() string {
	s := HubServerServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func ClairServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Clair)
}

func ClairServiceAddress() string {
	s := ClairServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func CollieServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Collie)
}

func CollieServiceAddress() string {
	s := CollieServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func WarpDriveServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.WarpDrive)
}

func WarpDriveServiceName() string {
	return WarpDriveServiceInfo().Name
}

func OPAServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.OPA)
}

func OPAServiceAddress() string {
	s := OPAServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func VendorServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Vendor)
}

func VendorServiceAddress() string {
	s := VendorServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func GetServiceAddress(name string, port int32) string {
	return fmt.Sprintf("http://%s:%d", name, port)
}

func MinioServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Minio)
}

func MinioServiceName() string {
	return MinioServiceInfo().Name
}

func DataPath() string {
	return "/app/data"
}

func VMTaskLogPath() string {
	return filepath.Join(DataPath(), "%vm-task%", "log")
}

func ObjectStorageServicePath(project, service string) string {
	return filepath.Join(project, "service", service)
}

func ObjectStorageProductionServicePath(project, service string) string {
	return filepath.Join(project, "production-service", service)
}

func ObjectStorageTemplatePath(name, kind string) string {
	return filepath.Join("templates", kind, name)
}

func ObjectStorageChartTemplatePath(name string) string {
	return ObjectStorageTemplatePath(name, setting.ChartTemplatesPath)
}

func LocalTestServicePath(project, service string) string {
	return filepath.Join(DataPath(), project, "test", service)
}

// LocalTestServicePathWithRevision returns a test service path with a given revision.
func LocalTestServicePathWithRevision(project, service, revision string) string {
	return filepath.Join(DataPath(), project, "test", service, revision)
}

// LocalProductionServicePathWithRevision returns a production service path with a given revision.
func LocalProductionServicePathWithRevision(project, service, revision string) string {
	return filepath.Join(DataPath(), project, "production", service, revision)
}

func LocalTemplatePath(name, kind string) string {
	return filepath.Join(DataPath(), "templates", kind, name)
}

func LocalChartTemplatePath(name string) string {
	return LocalTemplatePath(name, setting.ChartTemplatesPath)
}

// if this path is changed, must also change cleanCacheFiles in pkg/microservice/aslan/core/clean_cache_files.go
func LocalHtmlReportPath(projectName, workflowName, jobTaskName string, taskID int64) string {
	return filepath.Join(DataPath(), "project", projectName, "workflow", workflowName, "jobTask", jobTaskName, "task", fmt.Sprintf("%d", taskID), "html-report") + "/"
}

func MongoURI() string {
	return viper.GetString(setting.ENVMongoDBConnectionString)
}

func MongoDatabase() string {
	return viper.GetString(setting.ENVAslanDBName)
}

func PolicyDatabase() string {
	return MongoDatabase() + "_policy"
}

func MysqlUser() string {
	return viper.GetString(setting.ENVMysqlUser)
}

func MysqlUserDB() string {
	return viper.GetString(setting.ENVMysqlUserDB)
}

func MysqlPassword() string {
	return viper.GetString(setting.ENVMysqlPassword)
}

func MysqlHost() string {
	return viper.GetString(setting.ENVMysqlHost)
}

func MysqlDexDB() string {
	return viper.GetString(setting.ENVMysqlDexDB)
}

func MysqlUseDM() bool {
	return viper.GetBool(setting.ENVMysqlUseDM)
}

func Namespace() string {
	return viper.GetString(setting.ENVNamespace)
}

func PodIP() string {
	return viper.GetString(setting.ENVPodIP)
}

func RoleBindingNameFromUIDAndRole(uid string, role setting.RoleType, roleNamespace string) string {
	return fmt.Sprintf("%s-%s-%s", uid, role, roleNamespace)
}

func BuildResourceKey(resourceType, projectName, labelBinding string) string {
	return fmt.Sprintf("%s-%s-%s", resourceType, projectName, labelBinding)
}

func RedisHost() string {
	return viper.GetString(setting.ENVRedisHost)
}

func RedisPort() int {
	return viper.GetInt(setting.ENVRedisPort)
}

func RedisUserName() string {
	return viper.GetString(setting.ENVRedisUserName)
}

func RedisPassword() string {
	return viper.GetString(setting.ENVRedisPassword)
}

func RedisCommonCacheTokenDB() int {
	return viper.GetInt(setting.ENVRedisCommonCacheDB)
}

func LarkPluginID() string {
	return viper.GetString(setting.ENVLarkPluginID)
}

func LarkPluginSecret() string {
	return viper.GetString(setting.ENVLarkPluginSecret)
}
func LarkPluginAccessTokenType() int {
	return viper.GetInt(setting.ENVLarkPluginAccessTokenType)
}

func DisableKubeClientKeepAlive() bool {
	return viper.GetBool(setting.ENVDisableKubeClientKeepAlive)
}

func IsDocumentDB() bool {
	return viper.GetBool(setting.ENVIsDocumentDB)
}

func Home() string {
	return viper.GetString(setting.Home)
}
