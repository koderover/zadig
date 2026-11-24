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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/command/cmd/agent"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/command/cmd/start"
	agentconfig "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/config"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/helper/log"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/common"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/internal/register"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/daemon"
	"github.com/koderover/zadig/v2/pkg/cli/zadig-agent/pkg/updater"
	osutil "github.com/koderover/zadig/v2/pkg/cli/zadig-agent/util/os"
	globalconfig "github.com/koderover/zadig/v2/pkg/config"
	pkglog "github.com/koderover/zadig/v2/pkg/tool/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("server-url", "s", "", "zadig-server url")
	startCmd.Flags().StringP("token", "t", "", "agent token")
	startCmd.Flags().StringP("work-dir", "d", "", "agent work directory")
	startCmd.Flags().Bool("auto-start", true, "Configure boot startup on first registration; failure falls back to ordinary execution")
	startCmd.Flags().String("service-user", "", "Service account (defaults to current account; Windows LocalSystem must be explicit)")
	startCmd.Flags().Bool("password-stdin", false, "Read the Windows service account password from standard input")
}

var ExistCh chan struct{} = make(chan struct{})
var instanceLease *os.File
var skipStart bool
var startManaged bool

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start Zadig-agent",
	Long:  `Start Zadig-agent, if you use sudo zadig.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		workDir, err := cmd.Flags().GetString("work-dir")
		if err != nil {
			return fmt.Errorf("failed to get work-dir: %v", err)
		}

		serverUrl, err := cmd.Flags().GetString("server-url")
		if err != nil {
			return fmt.Errorf("failed to get server-url: %v", err)
		}

		token, err := cmd.Flags().GetString("token")
		if err != nil {
			return fmt.Errorf("failed to get token: %v", err)
		}

		skipStart, startManaged = false, false
		background := os.Getenv(agentconfig.BackgroundEnv) == "1"
		allowed, err := agentconfig.ConnectionAllowed()
		if err != nil {
			return err
		}
		if background && !allowed {
			skipStart = true
			return nil
		}
		if !background {
			if err := agentconfig.SetConnectionAllowed(true); err != nil {
				return err
			}
		}
		path, err := agentconfig.GetAgentConfigFilePath()
		if err != nil {
			return err
		}
		_, statErr := os.Stat(path)
		firstRegistration := errors.Is(statErr, os.ErrNotExist)
		if statErr != nil && !firstRegistration {
			return statErr
		}
		lease, err := updater.AcquireInstance()
		if err != nil {
			return fmt.Errorf("agent is already running or cannot acquire instance lock: %w", err)
		}
		if err := updater.CheckStartup(); err != nil {
			lease.Close()
			return err
		}
		if err := startPreRun(serverUrl, token, workDir); err != nil {
			lease.Close()
			if errors.Is(err, agentconfig.ErrDisconnected) {
				skipStart = true
				return nil
			}
			return err
		}
		instanceLease = lease
		if !background {
			requested, _ := cmd.Flags().GetBool("auto-start")
			if requested && (firstRegistration || cmd.Flags().Changed("auto-start")) {
				account, _ := cmd.Flags().GetString("service-user")
				options, err := serviceInstallOptions(cmd, account, true)
				if err == nil {
					_, err = installAgentService(cmd, options)
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Automatic boot startup could not be configured: %v. Continuing ordinary agent execution; after reboot, run start again.\n", err)
				}
			} else if !requested && cmd.Flags().Changed("auto-start") {
				if _, err := daemon.Load(); err == nil {
					if err := serviceOperation(cmd, "disable", func(context.Context) error { return daemon.SetAutoStart(false) }); err != nil {
						lease.Close()
						return err
					}
				}
			}
			if s, err := daemon.Load(); err == nil && s.Backend != daemon.Cron {
				startManaged = true
				lease.Close()
				instanceLease = nil
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if instanceLease != nil {
			defer instanceLease.Close()
		}
		if skipStart {
			return nil
		}
		if startManaged {
			return serviceOperation(cmd, "start", daemon.Start)
		}
		return startRun(cmd.Context(), func() {})
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if err := startPostRun(); err != nil {
			fmt.Printf("Start post run failed: %v.", err)
		}
	},
}

func startPreRun(serverURL, token, workDir string) error {
	// remove some running garbage
	resetAgentRunningFile()

	// init Viper
	// automatically load environment variables
	viper.AutomaticEnv()

	initPkgLog()

	if !agentconfig.InitConfig() {
		if os.Getenv(agentconfig.BackgroundEnv) == "1" {
			return fmt.Errorf("agent configuration is missing; background startup cannot perform interactive registration")
		}
		log.Infof("agent config file not found, start to register agent to zadig-server.")

		if err := start.AskInstaller(&serverURL, &token, &workDir); err != nil {
			return fmt.Errorf("failed to ask installer: %v", err)
		}

		viper.Set(common.AGENT_INSTALL_TIME, time.Now().Unix())
		viper.Set(common.AGENT_VERSION, agentconfig.BuildAgentVersion)
		user, err := osutil.GetOSCurrentUser()
		if err != nil {
			return err
		}
		viper.Set(common.AGENT_INSTALL_USER, user)

		return register.RegisterAgent()
	}

	if serverURL != "" {
		agentconfig.SetServerURL(serverURL)
	}

	if token != "" {
		agentconfig.SetAgentToken(token)
	}

	if workDir != "" {
		agentconfig.SetAgentWorkDirectory(workDir)
	}

	log.Infof("find the agent config file in local disk")
	return nil
}

func startRun(ctx context.Context, ready func()) error { return agent.StartAgent(ctx, ready) }

func startPostRun() error {
	return nil
}

func resetAgentRunningFile() {
	stopFilePath, err := agentconfig.GetStopFilePath()
	if err != nil {
		log.Errorf("failed to get stop file path: %v", err)
	}
	if _, err := os.Stat(stopFilePath); err == nil {
		err = os.Remove(stopFilePath)
		if err != nil {
			log.Errorf("failed to remove stop file: %v", err)
		}
	}
}

func initPkgLog() {
	path, err := agentconfig.GetAgentLogPath()
	if err != nil {
		panic(err)
	}

	pkglog.Init(&pkglog.Config{
		Level:      globalconfig.LogLevel(),
		Filename:   filepath.Join(path, "zadig-agent.log"),
		SendToFile: true,
	})
}
