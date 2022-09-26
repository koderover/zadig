package main

import (
	"fmt"
	"os"

	"version/utils"
)

func main() {

	args := os.Args
	if len(args) >= 2 && args[1] == "version" {
		v := utils.GetVersion()
		fmt.Printf("Version: %s\nBranch: %s\nCommit: %s\nPR: %s\nBuild Time: %s\nGo Version: %s\nOS/Arch: %s\nBuild URL: %s\n", v.Version, v.GitBranch, v.GitCommit,v.GitPR, v.BuildDate, v.GoVersion, v.Platform,v.BuildURL)
	} else {
		fmt.Printf("Version(hard code): %s\n", "0.1")
	}
}
