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

	"github.com/spf13/viper"

	"github.com/koderover/zadig/pkg/setting"
)

// SystemAddress is the fully qualified domain name of the system, or an IP Address.
// Port and protocol are required if necessary.
// for example: foo.bar.com, https://for.bar.com, http://1.2.3.4:5678
func SystemAddress() string {
	return viper.GetString(setting.ENVSystemAddress)
}

func Enterprise() bool {
	return viper.GetBool(setting.ENVEnterprise)
}

func Mode() string {
	mode := viper.GetString(setting.ENVMode)
	if mode == "" {
		return setting.DebugMode
	}

	return mode
}

func LogLevel() string {
	return "debug"
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

func PoetryAPIRootKey() string {
	return viper.GetString(setting.ENVPoetryAPIRootKey)
}

func GetServiceByCode(code int) *setting.ServiceInfo {
	return setting.Services[code]
}

func AslanServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Aslan)
}

func AslanServiceAddress() string {
	s := AslanServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func AslanServiceName() string {
	return AslanServiceInfo().Name
}

func AslanServicePort() int32 {
	return AslanServiceInfo().Port
}

func AslanxServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Aslanx)
}

func AslanxServiceAddress() string {
	s := AslanxServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func AslanxServiceName() string {
	return AslanxServiceInfo().Name
}

func AslanxServicePort() int32 {
	return AslanxServiceInfo().Port
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

func PoetryServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.Poetry)
}

func PoetryServiceAddress() string {
	s := PoetryServiceInfo()
	return GetServiceAddress(s.Name, s.Port)
}

func WarpDriveServiceInfo() *setting.ServiceInfo {
	return GetServiceByCode(setting.WarpDrive)
}

func WarpDriveServiceName() string {
	return WarpDriveServiceInfo().Name
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
