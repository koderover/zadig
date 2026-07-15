package llm

import (
	"errors"
	"time"
)

var ErrMaxTokensExceeded = errors.New("llm completion reached max tokens")

type ReasoningEffort string

const (
	ReasoningEffortLow    ReasoningEffort = "low"
	ReasoningEffortMedium ReasoningEffort = "medium"
	ReasoningEffortHigh   ReasoningEffort = "high"
)

// ParamOption is a function that configures a CallOptions.
type ParamOption func(*ParamOptions)

// ParamOptions is a set of options.
type ParamOptions struct {
	// Model is the model to use.
	Model string `json:"model"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// ReasoningEffort controls reasoning depth for providers that support it.
	ReasoningEffort ReasoningEffort `json:"reasoning_effort"`
	// ErrorOnMaxTokens returns ErrMaxTokensExceeded when generation reaches its token limit.
	ErrorOnMaxTokens bool `json:"error_on_max_tokens"`
	// RequestTimeout overrides the default timeout for this completion request.
	RequestTimeout time.Duration `json:"-"`
	// Temperature is the temperature for sampling, between 0 and 1.
	Temperature float32 `json:"temperature"`
	// StopWords is a list of words to stop on.
	StopWords []string       `json:"stop_words"`
	LogitBias map[string]int `json:"logit_bias"`
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

func WithReasoningEffort(reasoningEffort ReasoningEffort) ParamOption {
	return func(o *ParamOptions) {
		o.ReasoningEffort = reasoningEffort
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
