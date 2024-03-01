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

package os

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"runtime"

	"github.com/shirou/gopsutil/mem"
)

type PlatformParameters struct {
	IP            string `json:"ip"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CurrentUser   string `json:"current_user"`
	MemTotal      uint64 `json:"mem_total"`
	UsedMem       uint64 `json:"used_mem"`
	CpuNum        int    `json:"cpu_num"`
	DiskSpace     uint64 `json:"disk_space"`
	FreeDiskSpace uint64 `json:"free_disk_space"`
	Hostname      string `json:"hostname"`
}

func GetPlatformParameters() (*PlatformParameters, error) {
	parameters := &PlatformParameters{}

	ipv4, err := GetHostIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get host ip: %v", err)
	}
	parameters.IP = ipv4

	parameters.OS = GetOSName()
	parameters.Arch = GetOSArch()
	parameters.CpuNum = GetCpuNum()

	currentUser, err := GetHostCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("failed to get current user: %v", err)
	}
	parameters.CurrentUser = currentUser

	memTotal, err := GetHostTotalMem()
	if err != nil {
		return nil, fmt.Errorf("failed to get total memory: %v", err)
	}
	parameters.MemTotal = memTotal

	UsedMem, err := GetHostUsedMem()
	if err != nil {
		return nil, fmt.Errorf("failed to get used memory: %v", err)
	}
	parameters.UsedMem = UsedMem

	diskTotal, err := GetDiskSpace()
	if err != nil {
		return nil, fmt.Errorf("failed to get disk space: %v", err)
	}
	parameters.DiskSpace = diskTotal

	freeDisk, err := GetFreeDiskSpace()
	if err != nil {
		return nil, fmt.Errorf("failed to get disk free space: %v", err)
	}
	parameters.FreeDiskSpace = freeDisk

	hostname, err := GetHostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %v", err)
	}
	parameters.Hostname = hostname

	return parameters, nil
}

func IToi(before interface{}, after interface{}) error {
	b, err := json.Marshal(before)
	if err != nil {
		return fmt.Errorf("marshal task error: %v", err)
	}

	if err := json.Unmarshal(b, &after); err != nil {
		return fmt.Errorf("unmarshal task error: %v", err)
	}

	return nil
}

func GetOSName() string {
	return runtime.GOOS
}

func GetPlatform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func GetOSArch() string {
	return runtime.GOARCH
}

func GetHostTotalMem() (uint64, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return memInfo.Total, nil
}

func GetHostUsedMem() (uint64, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return memInfo.Used, nil
}

func GetCpuNum() int {
	return runtime.NumCPU()
}

func GetHostCurrentUser() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	return currentUser.Username, nil
}

func GetHostIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		fmt.Println(err)
	}
	var ip string
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ip = ipnet.IP.String()
			}
		}
	}
	return ip, nil
}

func GetHostname() (string, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return hostname, nil
}

func GetOSCurrentUser() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to get os current user: %s", err)
	}
	return currentUser.Username, nil
}
