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

package mongodb

import (
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	models2 "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/system/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/types"
)

// Large JSON/YAML bodies can consume substantially more memory while parsing.
// Worker creation is also bounded by GOMAXPROCS in the response path.
const operationLogMaskMaxConcurrency = 50

type operationLogSensitiveCandidate struct {
	value              string
	requiresTerminator bool
}

// Indexing candidates by their normalized first byte keeps the hot-path scan
// single-pass and allocation-free while making the keyword set declarative.
var operationLogSensitiveCandidates = [256][]operationLogSensitiveCandidate{
	'a': {{value: "api"}, {value: "access"}, {value: "ak", requiresTerminator: true}},
	'c': {{value: "credential"}, {value: "connection"}},
	'e': {{value: "encryption"}},
	'p': {{value: "password"}, {value: "passwd"}, {value: "pwd"}, {value: "private"}},
	's': {{value: "secret"}, {value: "sensitive"}, {value: "sk", requiresTerminator: true}},
	't': {{value: "token"}},
}

func sanitizeOperationLogsForResponse(operationLogs []*models2.OperationLog) {
	pending := 0
	for _, operationLog := range operationLogs {
		if operationLog != nil && !operationLog.RequestBodyMaskingProcessed {
			pending++
		}
	}
	if pending == 0 {
		return
	}

	workerCount := min(pending, operationLogMaskMaxConcurrency, runtime.GOMAXPROCS(0))

	jobs := make(chan *models2.OperationLog, workerCount)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go func() {
			defer workers.Done()
			for operationLog := range jobs {
				maskOperationLog(operationLog)
			}
		}()
	}

	for _, operationLog := range operationLogs {
		if operationLog != nil && !operationLog.RequestBodyMaskingProcessed {
			jobs <- operationLog
		}
	}
	close(jobs)
	workers.Wait()
}

func maskOperationLog(operationLog *models2.OperationLog) {
	if operationLog == nil || operationLog.RequestBodyMaskingProcessed {
		return
	}

	operationLog.RequestBodyMaskingProcessed = true

	operationLog.RequestBody = maskOperationLogRequestBody(operationLog.RequestBody, operationLog.BodyType)
}

func maskOperationLogRequestBody(requestBody string, bodyType types.RequestBodyType) string {
	if requestBody == "" || (bodyType != types.RequestBodyTypeJSON && bodyType != types.RequestBodyTypeYAML && bodyType != "") {
		return requestBody
	}

	// Parsing is only needed when masking may change the body. Otherwise the
	// request body is preserved verbatim, including malformed input.
	if !mayContainOperationLogSensitiveData(requestBody) {
		return requestBody
	}

	var masked string
	var changed bool
	var err error

	switch bodyType {
	case types.RequestBodyTypeJSON:
		masked, changed, err = maskOperationLogJSON(requestBody)
	case types.RequestBodyTypeYAML:
		masked, changed, err = maskOperationLogYAML(requestBody)
	default:
		masked, changed, err = maskOperationLogJSON(requestBody)
		if err != nil {
			masked, changed, err = maskOperationLogYAML(requestBody)
		}
	}
	if err != nil || !changed {
		return requestBody
	}
	return masked
}

func mayContainOperationLogSensitiveData(input string) bool {
	for offset := 0; offset < len(input); offset++ {
		for _, candidate := range operationLogSensitiveCandidates[asciiLower(input[offset])] {
			if matchesOperationLogSensitiveCandidate(input, offset, candidate) {
				return true
			}
		}
	}
	return false
}

func matchesOperationLogSensitiveCandidate(input string, offset int, candidate operationLogSensitiveCandidate) bool {
	if candidate.requiresTerminator {
		return hasShortCredentialSuffix(input, offset, candidate.value)
	}
	return hasASCIIFoldPrefix(input, offset, candidate.value)
}

func hasASCIIFoldPrefix(input string, offset int, candidate string) bool {
	if offset+len(candidate) > len(input) {
		return false
	}
	for i := 0; i < len(candidate); i++ {
		if asciiLower(input[offset+i]) != candidate[i] {
			return false
		}
	}
	return true
}

func hasShortCredentialSuffix(input string, offset int, candidate string) bool {
	if offset > 0 && isASCIIAlphaNumeric(input[offset-1]) && !isOperationLogCamelCaseBoundary(input, offset) {
		return false
	}
	if !hasASCIIFoldPrefix(input, offset, candidate) {
		return false
	}

	next := offset + len(candidate)
	if next == len(input) {
		return true
	}
	switch input[next] {
	case '"', '\'', ':', '_', '-', '.', ' ', '\t', '\r', '\n', ',', '}', ']':
		return true
	default:
		return false
	}
}

func isOperationLogCamelCaseBoundary(input string, offset int) bool {
	current, previous := input[offset], input[offset-1]
	return current >= 'A' && current <= 'Z' &&
		(previous >= 'a' && previous <= 'z' || previous >= '0' && previous <= '9')
}

func asciiLower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func maskOperationLogJSON(requestBody string) (string, bool, error) {
	decoder := json.NewDecoder(strings.NewReader(requestBody))
	decoder.UseNumber()

	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return "", false, err
	}
	if err := ensureJSONDecoderEOF(decoder); err != nil {
		return "", false, err
	}

	if !maskOperationLogValue(value) {
		return requestBody, false, nil
	}
	masked, err := json.Marshal(value)
	if err != nil {
		return "", false, err
	}
	return string(masked), true, nil
}

func ensureJSONDecoderEOF(decoder *json.Decoder) error {
	if _, err := decoder.Token(); err == nil {
		return errors.New("multiple JSON values")
	} else if err != io.EOF {
		return err
	}
	return nil
}

func maskOperationLogValue(value interface{}) bool {
	switch typedValue := value.(type) {
	case map[string]interface{}:
		sensitiveNode := isOperationLogSensitiveNode(typedValue)
		changed := false
		for key, item := range typedValue {
			if key == "variable_yaml" {
				if embeddedYAML, ok := item.(string); ok {
					masked := maskOperationLogRequestBody(embeddedYAML, types.RequestBodyTypeYAML)
					if masked != embeddedYAML {
						typedValue[key] = masked
						changed = true
					}
					continue
				}
			}

			if isSensitiveOperationLogIdentifier(key) || sensitiveNode && isOperationLogSensitiveValueKey(key) {
				if shouldMaskOperationLogFieldValue(item) {
					typedValue[key] = setting.MaskValue
					changed = true
				}
				continue
			}

			if maskOperationLogValue(item) {
				changed = true
			}
		}
		return changed
	case []interface{}:
		changed := false
		for _, item := range typedValue {
			if maskOperationLogValue(item) {
				changed = true
			}
		}
		return changed
	default:
		return false
	}
}

func isOperationLogSensitiveNode(value map[string]interface{}) bool {
	for _, flag := range []string{"is_credential", "is_sensitive"} {
		if enabled, ok := value[flag].(bool); ok && enabled {
			return true
		}
	}

	for _, identifierKey := range []string{"key", "name", "variable_key"} {
		if identifier, ok := value[identifierKey].(string); ok && isSensitiveOperationLogIdentifier(identifier) {
			return true
		}
	}
	return false
}

func shouldMaskOperationLogFieldValue(value interface{}) bool {
	switch typedValue := value.(type) {
	case nil, bool:
		return false
	case string:
		return typedValue != "" && typedValue != setting.MaskValue
	case []interface{}:
		return len(typedValue) != 0
	case map[string]interface{}:
		return len(typedValue) != 0
	default:
		return true
	}
}

func maskOperationLogYAML(requestBody string) (string, bool, error) {
	var value interface{}
	if err := yaml.NewDecoder(strings.NewReader(requestBody)).Decode(&value); err != nil {
		return "", false, err
	}
	if !maskOperationLogValue(value) {
		return requestBody, false, nil
	}
	masked, err := yaml.Marshal(value)
	if err != nil {
		return "", false, err
	}
	return string(masked), true, nil
}

func isOperationLogSensitiveValueKey(key string) bool {
	normalized := normalizeOperationLogIdentifier(key)
	return normalized == "value" || normalized == "default" || normalized == "choice_value"
}

func isSensitiveOperationLogIdentifier(identifier string) bool {
	normalized := normalizeOperationLogIdentifier(identifier)
	if normalized == "" {
		return false
	}
	if normalized == "encryption" {
		return true
	}

	for _, suffix := range []string{"password", "passwd", "token", "secret", "credential"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	for _, suffix := range []string{"pwd", "ak", "sk"} {
		if hasNormalizedOperationLogSuffix(normalized, suffix) {
			return true
		}
	}
	for _, suffix := range []string{"api_key", "access_key", "access_key_id", "secret_key", "private_key", "connection_string", "apikey", "accesskey", "accesskeyid", "secretkey", "privatekey", "connectionstring"} {
		if hasNormalizedOperationLogSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

func hasNormalizedOperationLogSuffix(identifier, suffix string) bool {
	return identifier == suffix || strings.HasSuffix(identifier, "_"+suffix)
}

func normalizeOperationLogIdentifier(identifier string) string {
	if identifier == "" {
		return ""
	}
	alreadyNormalized := true
	for i := 0; i < len(identifier); i++ {
		current := identifier[i]
		if (current >= 'a' && current <= 'z') || (current >= '0' && current <= '9') {
			continue
		}
		if current != '_' || i == 0 || i+1 == len(identifier) || identifier[i-1] == '_' {
			alreadyNormalized = false
			break
		}
	}
	if alreadyNormalized {
		return identifier
	}

	var builder strings.Builder
	builder.Grow(len(identifier) + 4)
	lastSeparator := true
	for i := 0; i < len(identifier); i++ {
		current := identifier[i]
		if !isASCIIAlphaNumeric(current) {
			if !lastSeparator && builder.Len() > 0 {
				builder.WriteByte('_')
				lastSeparator = true
			}
			continue
		}

		if current >= 'A' && current <= 'Z' {
			previousIsLowerOrDigit := i > 0 && (identifier[i-1] >= 'a' && identifier[i-1] <= 'z' || identifier[i-1] >= '0' && identifier[i-1] <= '9')
			nextIsLower := i+1 < len(identifier) && identifier[i+1] >= 'a' && identifier[i+1] <= 'z'
			previousIsUpper := i > 0 && identifier[i-1] >= 'A' && identifier[i-1] <= 'Z'
			if !lastSeparator && (previousIsLowerOrDigit || previousIsUpper && nextIsLower) {
				builder.WriteByte('_')
			}
			current = asciiLower(current)
		}
		builder.WriteByte(current)
		lastSeparator = false
	}

	normalized := builder.String()
	return strings.TrimSuffix(normalized, "_")
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}
