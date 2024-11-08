/*
Copyright 2024 The KodeRover Authors.

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

package perforce

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// PerforceLogin returns command:
// - p4 set P4PORT=$P4PORT
// - p4 set P4USER=$P4USER
// - p4 login (with interactive login password)
func PerforceLogin(host string, port int, username, password string) []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)

	p4Connection := fmt.Sprintf("P4Port=%s:%d", host, port)

	cmds = append(cmds, exec.Command(
		"p4",
		"set",
		p4Connection,
	))

	cmds = append(cmds, exec.Command(
		"p4",
		"set",
		fmt.Sprintf("P4USER=%s", username),
	))

	loginCmd := exec.Command(
		"p4",
		"login",
	)

	var stdin bytes.Buffer
	stdin.WriteString(password + "\n")
	loginCmd.Stdin = &stdin

	cmds = append(cmds, loginCmd)

	return cmds
}

const streamTypeConfigFile = `Client: %s
Root: /workspace/%s
Type: writable
Stream: %s`

const localTypeConfigFile = `Client: %s
Root: /workspace
View:
%s`

// PerforceCreateWorkspace returns workspace creation command, the config file depends on the perforce depot type:
// the command to run is:
// - p4 client (with interactive config file input)
//
// config file will be as follow
// =================
// |   stream type   |
// =================
// Client: <client-name>
// Root: /workspace/<depotName>
// Type: writeable
// Stream: <user provided stream info>
//
// =================
// |   local type    |
// =================
// Client: <client-name>
// Root: /workspace
// View:
//
//	<User provided view mapping>
func PerforceCreateWorkspace(clientName, depotType, streamInfo, viewMapping string) []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)

	var configFile string
	switch depotType {
	case "stream":
		// get the depotName
		segs := strings.Split(strings.TrimPrefix(streamInfo, "//"), "/")
		depotName := segs[0]
		configFile = fmt.Sprintf(streamTypeConfigFile, clientName, depotName, streamInfo)
	case "local":
		mappings := strings.Split(viewMapping, "\n")
		viewMappingStr := ""
		for _, mapping := range mappings {
			// now we support only $P4CLIENT as a special variable
			viewMappingStr += fmt.Sprintf("    %s\n", strings.Replace(mapping, "$P4CLIENT", clientName, -1))
		}
		configFile = fmt.Sprintf(localTypeConfigFile, clientName, viewMappingStr)
	default:
		return cmds
	}

	configWorkspaceCommand := exec.Command(
		"p4",
		"client",
		"-i",
	)

	var stdin bytes.Buffer
	stdin.WriteString(configFile)
	configWorkspaceCommand.Stdin = &stdin

	cmds = append(cmds, configWorkspaceCommand)

	return cmds
}

// PerforceSync returns perforce sync command with given changelistID and clientName. If the id is 0, fetch the latest
// sample command: p4 -c <clientName> sync @<changelistID>
func PerforceSync(clientName string, changelistID int) []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)

	if changelistID == 0 {
		cmds = append(cmds, exec.Command(
			"p4",
			"sync",
			"-c",
			clientName,
		))
	} else {
		cmds = append(cmds, exec.Command(
			"p4",
			"-c",
			clientName,
			"sync",
			fmt.Sprintf("@%d", changelistID),
		))
	}

	return cmds
}

// PerforceUnshelve returns perforce unshelve command with given changelistID and shelveID. If the id is 0, return nothing
// sample command: p4 unshelve -s <shelveID>
func PerforceUnshelve(clientName string, shelveID int) []*exec.Cmd {
	cmds := make([]*exec.Cmd, 0)

	if shelveID == 0 {
		return cmds
	} else {
		cmds = append(cmds, exec.Command(
			"p4",
			"-c",
			clientName,
			"unshelve",
			"-s",
			fmt.Sprintf("%d", shelveID),
		))
	}

	return cmds
}
