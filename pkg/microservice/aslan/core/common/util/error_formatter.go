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

package util

import (
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"

	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

// FormatCodeHostError formats code host errors into generic user-friendly messages.
// It identifies common error types (401, 403, 404, 429, timeout, etc.) and
// returns appropriate descriptions for frontend display.
func FormatCodeHostError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		return "Code host is not configured or does not exist"
	}

	var httpErr *e.HTTPError
	if errors.As(err, &httpErr) && httpErr.Desc() != "" {
		raw := strings.ToLower(httpErr.Desc())
		if msg := matchErrorPattern(raw, "Resource not found"); msg != "" {
			return msg
		}
	}

	raw := strings.ToLower(err.Error())
	if msg := matchErrorPattern(raw, "Resource not found"); msg != "" {
		return msg
	}

	return ""
}

// FormatRepoInfoError formats repo info lookup errors into repo-specific user-facing messages.
func FormatRepoInfoError(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, mongo.ErrNoDocuments) {
		return "Code host is not configured or does not exist"
	}

	var httpErr *e.HTTPError
	if errors.As(err, &httpErr) && httpErr.Desc() != "" {
		raw := strings.ToLower(httpErr.Desc())
		if msg := matchErrorPattern(raw, "Repository not found or access denied"); msg != "" {
			return msg
		}
	}

	raw := strings.ToLower(err.Error())
	if msg := matchErrorPattern(raw, "Repository not found or access denied"); msg != "" {
		return msg
	}

	return "Failed to fetch repository metadata"
}

// FormatCodeHostErrorWithDefault formats code host errors with a default description.
// It appends the specific error detail to the default description.
func FormatCodeHostErrorWithDefault(defaultDesc string, err error) string {
	if err == nil {
		return defaultDesc
	}

	detail := FormatCodeHostError(err)
	if detail == "" {
		return defaultDesc
	}

	base := strings.TrimSpace(strings.TrimRight(defaultDesc, "."))
	return fmt.Sprintf("%s. %s", base, detail)
}

// matchErrorPattern matches common error patterns and returns user-friendly messages.
func matchErrorPattern(raw, notFoundMsg string) string {
	switch {
	case strings.Contains(raw, "401"), strings.Contains(raw, "unauthorized"):
		return "Authentication failed, please check the access credentials"
	case strings.Contains(raw, "403"), strings.Contains(raw, "forbidden"):
		return "Permission denied"
	case strings.Contains(raw, "404"), strings.Contains(raw, "not found"):
		return notFoundMsg
	case strings.Contains(raw, "429"), strings.Contains(raw, "rate limit"):
		return "Rate limit exceeded"
	case strings.Contains(raw, "deadline exceeded"), strings.Contains(raw, "timeout"):
		return "Connection timeout"
	case strings.Contains(raw, "no documents"), strings.Contains(raw, "not exist"):
		return "Code host is not configured or does not exist"
	default:
		return ""
	}
}
