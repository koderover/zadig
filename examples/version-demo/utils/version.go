package utils

import (
	"fmt"
	"runtime"
)

var (
	version      string
	gitBranch    string
	gitTag       string
	gitCommit    string
	gitPR        string
	gitTreeState string
	buildDate    string
	buildURL      string
)

// Info contains versioning information.
type Info struct {
	Version      string `json:"version"`
	GitBranch    string `json:"gitBranch"`
	GitTag       string `json:"gitTag"`
	GitCommit    string `json:"gitCommit"`
	GitPR        string `json:"gitPR"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	BuildURL     string `json:"buildURL"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	return info.Platform
}

func GetVersion() Info {
	return Info{
		Version:      version,
		GitBranch:    gitBranch,
		GitTag:       gitTag,
		GitCommit:    gitCommit,
		GitPR:        gitPR,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		BuildURL:     buildURL,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
