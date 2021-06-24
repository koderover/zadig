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

func trace(cmd *exec.Cmd) {
	fmt.Printf("%s + %s\n", timeStamp(), strings.Join(cmd.Args, " "))
}

func info(msg string) {
	fmt.Printf("%s + %s\n", timeStamp(), msg)
}

func warning(msg string) {
	fmt.Printf("%s + [WARN] %s\n", timeStamp(), msg)
}

func timeStamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
