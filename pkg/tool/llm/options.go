package llm

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

var (
	ErrMaxTokensExceeded       = errors.New("llm completion reached max tokens")
	ErrEmptyCompletionResponse = errors.New("llm completion returned no usable response")
	ErrInvalidCompletion       = errors.New("llm completion returned an invalid response")
)

type completionHTTPError struct {
	statusCode int
	err        error
}

func (e *completionHTTPError) Error() string { return e.err.Error() }
func (e *completionHTTPError) Unwrap() error { return e.err }

func newCompletionHTTPError(statusCode int, err error) error {
	return &completionHTTPError{statusCode: statusCode, err: err}
}

type completionResponseError struct {
	kind error
	err  error
}

func (e *completionResponseError) Error() string { return e.err.Error() }
func (e *completionResponseError) Unwrap() error { return e.err }
func (e *completionResponseError) Is(target error) bool {
	return target == e.kind || errors.Is(e.err, target)
}

func newCompletionResponseError(kind, err error) error {
	return &completionResponseError{kind: kind, err: err}
}

func IsRetryableCompletionError(err error) bool {
	if errors.Is(err, ErrMaxTokensExceeded) ||
		errors.Is(err, ErrEmptyCompletionResponse) ||
		errors.Is(err, ErrInvalidCompletion) ||
		errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var httpErr *completionHTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	return httpErr.statusCode == http.StatusRequestTimeout ||
		httpErr.statusCode == http.StatusConflict ||
		httpErr.statusCode == http.StatusTooEarly ||
		httpErr.statusCode == http.StatusTooManyRequests ||
		httpErr.statusCode >= http.StatusInternalServerError
}

// ParamOption is a function that configures a CallOptions.
type ParamOption func(*ParamOptions)

// ParamOptions is a set of options.
type ParamOptions struct {
	// Model is the model to use.
	Model string `json:"model"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// ErrorOnMaxTokens returns ErrMaxTokensExceeded when generation reaches its token limit.
	ErrorOnMaxTokens bool `json:"error_on_max_tokens"`
	// RequestTimeout overrides the default timeout for this completion request.
	RequestTimeout time.Duration `json:"-"`
	// Temperature is the temperature for sampling, between 0 and 1.
	Temperature float32 `json:"temperature"`
	// StopWords is a list of words to stop on.
	StopWords []string       `json:"stop_words"`
	LogitBias map[string]int `json:"logit_bias"`

	temperatureSet bool
}

func WithModel(model string) ParamOption {
	return func(o *ParamOptions) {
		o.Model = model
	}
}

func WithMaxTokens(maxTokens int) ParamOption {
	return func(o *ParamOptions) {
		o.MaxTokens = maxTokens
	}
}

func WithErrorOnMaxTokens() ParamOption {
	return func(o *ParamOptions) {
		o.ErrorOnMaxTokens = true
	}
}

func WithRequestTimeout(timeout time.Duration) ParamOption {
	return func(o *ParamOptions) {
		o.RequestTimeout = timeout
	}
}

func WithTemperature(temperature float32) ParamOption {
	return func(o *ParamOptions) {
		o.Temperature = temperature
		o.temperatureSet = true
	}
}

func WithStopWords(stopWords []string) ParamOption {
	return func(o *ParamOptions) {
		o.StopWords = stopWords
	}
}

func WithLogitBias(logitBias map[string]int) ParamOption {
	return func(o *ParamOptions) {
		o.LogitBias = logitBias
	}
}

func WithOptions(options ParamOptions) ParamOption {
	return func(o *ParamOptions) {
		(*o) = options
	}
}

func ValidOptions(options ParamOptions) ParamOptions {
	if len(options.StopWords) == 0 {
		options.StopWords = nil
	}
	return options
}

func (o ParamOptions) hasTemperature() bool {
	return o.temperatureSet || o.Temperature != 0
}
