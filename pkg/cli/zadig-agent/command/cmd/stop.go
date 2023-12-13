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

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop Zadig-agent",
	Long:  `Stop Zadig-agent.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return stopPreRun()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := stopRun(); err != nil {
			fmt.Printf("Stop failed: %v.", err)
		}
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := stopPostRun(); err != nil {
			fmt.Printf("Stop post run failed: %v.", err)
		}
	},
}

func stopPreRun() error {
	return nil
}

func stopRun() error {
	stop, err := config.GetStopFilePath()
	if err != nil {
		return fmt.Errorf("failed to get stop file path: %v", err)
	}
	if _, err := os.Stat(stop); err == nil {
		err = os.Remove(stop)
		if err != nil {
			return fmt.Errorf("failed to remove stop file: %v", err)
		}
	}
	file, err := os.Create(stop)
	if err != nil {
		return fmt.Errorf("failed to create stop file: %v", err)
	}
	defer file.Close()

	// write stop signal to file
	_, err = file.WriteString("stop")
	if err != nil {
		return fmt.Errorf("failed to write stop signal to file: %v", err)
	}
	return nil
}

func stopPostRun() error {
	return nil
}
