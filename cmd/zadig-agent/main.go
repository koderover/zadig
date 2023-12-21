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

package main

import (
	"os"
	"runtime/debug"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/command"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
)

var (
	BuildAgentVersion = ""
	BuildGoVersion    = ""
	BuildCommit       = ""
	BuildTime         = ""
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			log.Errorf("Agent panic error: %v", err)
			log.Errorf("Agent panic stack: %s", string(debug.Stack()))
			os.Exit(1)
		}
	}()

	config.BuildAgentVersion = BuildAgentVersion
	config.BuildGoVersion = BuildGoVersion
	config.BuildCommit = BuildCommit
	config.BuildTime = BuildTime

	if err := command.Execute(); err != nil {
		log.Fatalf("Failed to run zadig-agent cmd executor, error: %s", err)
	}
}
