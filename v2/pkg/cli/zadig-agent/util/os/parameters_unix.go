//go:build linux || darwin
// +build linux darwin

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
	"syscall"
)

func GetDiskSpace() (uint64, error) {
	var totalSize uint64
	var stat syscall.Statfs_t

	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, err
	}
	// 文件系统块大小和可用块数的乘积即为总大小
	totalSize = stat.Blocks * uint64(stat.Bsize)

	return totalSize, nil
}

func GetFreeDiskSpace() (uint64, error) {
	var freeSize uint64
	var stat syscall.Statfs_t

	err := syscall.Statfs("/", &stat)
	if err != nil {
		return 0, err
	}
	// 文件系统块大小和可用块数的乘积即为总大小
	freeSize = stat.Bfree * uint64(stat.Bsize)

	return freeSize, nil
}
