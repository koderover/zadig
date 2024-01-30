package main

import (
	"os"

	"github.com/spf13/pflag"
	"k8s.io/component-base/cli/flag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	"github.com/koderover/demo/cmd/myapp-1/app"
	"github.com/koderover/demo/pkg/version"
)

func main() {
	op := app.NewOptions()
	op.AddFlags(pflag.CommandLine)

	flag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if op.PrintVersion {
		klog.InfoS("Version", "version", version.Version)
		os.Exit(0)
	}

	if err := app.Run(op); err != nil {
		klog.ErrorS(err, "Failed to run app")
		os.Exit(1)
	}
}
