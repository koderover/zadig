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

package common

// config const
const (
	ZADIG_SERVER_URL     = "ZADIG_SERVER_URL"
	AGENT_TOKEN          = "AGENT_TOKEN"
	AGENT_STATUS         = "AGENT_STATUS"
	AGENT_DESCRIPTION    = "AGENT_DESCRIPTION"
	AGENT_CONCURRENCY    = "AGENT_CONCURRENCY"
	AGENT_VERSION        = "ZADIG_AGENT_VERSION"
	ZADIG_VERSION        = "ZADIG_VERSION"
	AGENT_INSTALL_TIME   = "AGENT_INSTALL_TIME"
	AGENT_INSTALL_USER   = "AGENT_INSTALL_USER"
	AGENT_UPDATE_TIME    = "AGENT_UPDATE_TIME"
	AGENT_UPDATE_USER    = "AGENT_UPDATE_USER"
	AGENT_WORK_DIRECTORY = "AGENT_WORK_DIRECTORY"
)

// agent status
const (
	AGENT_STATUS_INIT        = "init"
	AGENT_STATUS_REGISTERING = "registering"
	AGENT_STATUS_REGISTERED  = "registered"
	AGENT_STATUS_RUNNING     = "running"
	AGENT_STATUS_ABNORMAL    = "abnormal"
	AGENT_STATUS_UPDATING    = "updating"
	AGENT_STATUS_STOP        = "stop"
)

type Status string

func (s Status) String() string {
	return string(s)
}

// zadig job status
const (
	StatusDisabled    Status = "disabled"
	StatusCreated     Status = "created"
	StatusRunning     Status = "running"
	StatusPassed      Status = "passed"
	StatusSkipped     Status = "skipped"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusCancelled   Status = "cancelled"
	StatusWaiting     Status = "waiting"
	StatusQueued      Status = "queued"
	StatusBlocked     Status = "blocked"
	QueueItemPending  Status = "pending"
	StatusChanged     Status = "changed"
	StatusNotRun      Status = "notRun"
	StatusPrepare     Status = "prepare"
	StatusReject      Status = "reject"
	StatusDistributed Status = "distributed"
)

const (
	DefaultAgentConcurrency          = 10
	DefaultAgentConcurrencyBlockTime = 5
	DefaultAgentPollingInterval      = 3
	DefaultJobReportInterval         = 1
	DefaultJobLogReadNum             = 100
)

const (
	LogLevelDebug   = "debug"
	LogLevelInfo    = "info"
	LogLevelWarning = "warning"
	LogLevelError   = "error"
	LogLevelFatal   = "fatal"
)

// config const
const (
	JobOutputDir     = "zadig/results/"
	Home             = "HOME"
	JobLogTmpDir     = "/tmp/job-log/"
	JobScriptTmpDir  = "/tmp/job-script/"
	JobOutputsTmpDir = "/tmp/job-outputs/"
	JobCacheTmpDir   = "/tmp/caches"
)

const (
	// MaxContainerTerminationMessageLength is the upper bound any one container may write to
	// its termination message path. Contents above this length will cause a failure.
	MaxContainerTerminationMessageLength = 1024 * 4
)

const (
	CacheDirWorkspaceType  = "workspace"
	CacheDirUserDefineType = "user_defined"
)
