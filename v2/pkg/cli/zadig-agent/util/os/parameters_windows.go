//go:build windows
// +build windows

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
	"fmt"
	"log"
	"syscall"
	"unsafe"
)

func GetDiskSpace() (uint64, error) {
	_, total, _, err := getDiskSpace()
	if err != nil {
		return 0, fmt.Errorf("get disk space failed: %v", err)
	}

	return total, nil
}

func GetFreeDiskSpace() (uint64, error) {
	_, _, free, err := getDiskSpace()
	if err != nil {
		return 0, fmt.Errorf("get disk space failed: %v", err)
	}

	return free, nil
}

func getDiskSpace() (uint64, uint64, uint64, error) {
	kernel32, err := syscall.LoadLibrary("Kernel32.dll")
	if err != nil {
		log.Panic(err)
	}
	defer syscall.FreeLibrary(kernel32)
	GetDiskFreeSpaceEx, err := syscall.GetProcAddress(syscall.Handle(kernel32), "GetDiskFreeSpaceExW")

	if err != nil {
		log.Panic(err)
	}

	available := uint64(0)
	total := uint64(0)
	free := uint64(0)

	r1, _, e1 := syscall.Syscall6(uintptr(GetDiskFreeSpaceEx), 4,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("C:"))),
		uintptr(unsafe.Pointer(&available)),
		uintptr(unsafe.Pointer(&total)),
		uintptr(unsafe.Pointer(&free)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
		return 0, 0, 0, err
	}

	return available, total, free, nil
}
