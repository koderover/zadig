package llm

// ParamOption is a function that configures a CallOptions.
type ParamOption func(*ParamOptions)

// ParamOptions is a set of options.
type ParamOptions struct {
	// Model is the model to use.
	Model string `json:"model"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"max_tokens"`
	// Temperature is the temperature for sampling, between 0 and 1.
	Temperature float32 `json:"temperature"`
	// StopWords is a list of words to stop on.
<<<<<<< HEAD
	StopWords []string       `json:"stop_words"`
	LogitBias map[string]int `json:"logit_bias"`
=======
	StopWords []string `json:"stop_words"`
>>>>>>> 3bdd9f0e1 (add llm and analyzer modules (#2723))
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

<<<<<<< HEAD
func WithLogitBias(logitBias map[string]int) ParamOption {
	return func(o *ParamOptions) {
		o.LogitBias = logitBias
	}
}

=======
>>>>>>> 3bdd9f0e1 (add llm and analyzer modules (#2723))
func WithOptions(options ParamOptions) ParamOption {
	return func(o *ParamOptions) {
		(*o) = options
	}
}

func ValidOptions(options ParamOptions) ParamOptions {
<<<<<<< HEAD
=======
	if options.MaxTokens == 0 {
		options.MaxTokens = 4096
	}
>>>>>>> 3bdd9f0e1 (add llm and analyzer modules (#2723))
	if len(options.StopWords) == 0 {
		options.StopWords = nil
	}
	return options
}
