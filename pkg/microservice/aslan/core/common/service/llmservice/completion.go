/*
Copyright 2026 The KodeRover Authors.

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

package llmservice

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/koderover/zadig/v2/pkg/tool/llm"
)

// CompleteWithRetry retries completion, empty response, and parse failures unless the request is canceled or times out.
func CompleteWithRetry[T any](
	ctx context.Context,
	client llm.ILLM,
	prompt string,
	maxRetries int,
	optionsForAttempt func(attempt int) []llm.ParamOption,
	parse func(answer string) (T, error),
) (T, string, error) {
	var result T
	var answer string
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return result, answer, err
		}

		answer, err := client.GetCompletion(ctx, prompt, optionsForAttempt(attempt)...)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return result, answer, err
			}
			lastErr = fmt.Errorf("llm completion failed: %w", err)
			continue
		}
		if strings.TrimSpace(answer) == "" {
			lastErr = errors.New("llm completion returned empty response")
			continue
		}

		parsed, err := parse(answer)
		if err == nil {
			return parsed, answer, nil
		}
		lastErr = fmt.Errorf("parse llm result failed: %w", err)
	}
	return result, answer, lastErr
}

// ExtractJSONCodeBlock removes an optional Markdown fence from a JSON response.
func ExtractJSONCodeBlock(text string) string {
	trimmed := strings.TrimSpace(text)
	if strings.HasPrefix(trimmed, "```json") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
		return strings.TrimSpace(trimmed)
	}
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
		if strings.HasSuffix(trimmed, "```") {
			trimmed = strings.TrimSuffix(trimmed, "```")
		}
	}
	return strings.TrimSpace(trimmed)
}
