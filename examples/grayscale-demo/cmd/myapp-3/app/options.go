package app

import (
	"github.com/spf13/pflag"
)

// Options is the configuration of demo.
type Options struct {
	ListenAddr   string
	PrintVersion bool
}

// NewOptions news configurations of demo.
func NewOptions() *Options {
	return &Options{}
}

// AddFlags adds flag options.
func (op *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&op.ListenAddr, "listen-addr", "0.0.0.0:8082", "listen addr")
	fs.BoolVar(&op.PrintVersion, "version", false, "Print version info and quit")
}
