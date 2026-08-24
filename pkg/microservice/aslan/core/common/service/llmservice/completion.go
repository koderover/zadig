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
	"time"

	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/tool/log"
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
		attemptStartedAt := time.Now()
		attemptNumber := attempt + 1
		totalAttempts := maxRetries + 1
		if err := ctx.Err(); err != nil {
			log.Warnf("llm completion stopped before attempt: integration=%s model=%s attempt=%d/%d err=%v", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, err)
			return result, answer, err
		}

		log.Infof("llm completion attempt started: integration=%s model=%s attempt=%d/%d prompt_bytes=%d", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, len(prompt))
		answer, err := client.GetCompletion(ctx, prompt, optionsForAttempt(attempt)...)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				log.Warnf("llm completion attempt canceled: integration=%s model=%s attempt=%d/%d duration=%s context_err=%v err=%v", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, time.Since(attemptStartedAt).Round(time.Millisecond), ctx.Err(), err)
				return result, answer, err
			}
			lastErr = fmt.Errorf("llm completion failed: %w", err)
			log.Warnf("llm completion attempt failed: integration=%s model=%s attempt=%d/%d duration=%s will_retry=%t err=%v", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, time.Since(attemptStartedAt).Round(time.Millisecond), attempt < maxRetries, lastErr)
			continue
		}
		if strings.TrimSpace(answer) == "" {
			lastErr = errors.New("llm completion returned empty response")
			log.Warnf("llm completion attempt returned empty response: integration=%s model=%s attempt=%d/%d duration=%s will_retry=%t", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, time.Since(attemptStartedAt).Round(time.Millisecond), attempt < maxRetries)
			continue
		}

		parsed, err := parse(answer)
		if err == nil {
			log.Infof("llm completion attempt succeeded: integration=%s model=%s attempt=%d/%d duration=%s response_bytes=%d", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, time.Since(attemptStartedAt).Round(time.Millisecond), len(answer))
			return parsed, answer, nil
		}
		lastErr = fmt.Errorf("parse llm result failed: %w", err)
		log.Warnf("llm completion attempt returned invalid result: integration=%s model=%s attempt=%d/%d duration=%s response_bytes=%d will_retry=%t err=%v", client.GetName(), client.GetModel(), attemptNumber, totalAttempts, time.Since(attemptStartedAt).Round(time.Millisecond), len(answer), attempt < maxRetries, lastErr)
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
