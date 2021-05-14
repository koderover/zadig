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

package service

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Trace writes the command in the programs stdout for debug purposes.
// the command is wrapped in xml tags for easy parsing.
func Trace(cmd *exec.Cmd) {
	fmt.Printf("%s + %s\n", timeStamp(), strings.Join(cmd.Args, " "))
}

// Info ...
func Info(msg string) {
	fmt.Printf("%s + %s\n", timeStamp(), msg)
}

// Infof ...
func Infof(format string, a ...interface{}) {
	format = fmt.Sprintf("%s + %s\n", timeStamp(), format)
	fmt.Printf(format, a...)
}

// Warning ...
func Warning(msg string) {
	fmt.Printf("%s + [WARN] %s\n", timeStamp(), msg)
}

// Warningf ...
func Warningf(format string, a ...interface{}) {
	format = fmt.Sprintf("%s + [WARN] %s\n", timeStamp(), format)
	fmt.Printf(format, a...)
}

// Error ...
func Error(msg string) {
	fmt.Printf("%s + [ERROR] %s\n", timeStamp(), msg)
}

// Errorf ...
func Errorf(format string, a ...interface{}) {
	format = fmt.Sprintf("%s + [ERROR] %s\n", timeStamp(), format)
	fmt.Printf(format, a...)
}

func timeStamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
