package app

import (
	"net/http"

	"github.com/spf13/pflag"
)

// Options is the configuration of demo.
type Options struct {
	ListenAddr     string
	DownstreamAddr string
	HttpClient     *http.Client
	Headers        []string

	PrintVersion bool
}

// NewOptions news configurations of demo.
func NewOptions() *Options {
	return &Options{
		HttpClient: &http.Client{},
	}
}

// AddFlags adds flag options.
func (op *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&op.ListenAddr, "listen-addr", "0.0.0.0:8081", "listen addr")
	fs.StringVar(&op.DownstreamAddr, "downstream-addr", "0.0.0.0:8082", "addr of downstream")
	fs.StringSliceVar(&op.Headers, "headers", []string{}, "http headers to propagate, such as 'x-name,x-age' or 'x-name'")

	fs.BoolVar(&op.PrintVersion, "version", false, "Print version info and quit")
}
