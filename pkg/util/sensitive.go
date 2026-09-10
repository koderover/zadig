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

import "strings"

// IsSensitiveKey identifies credential fields using the operation log rules.
func IsSensitiveKey(key string) bool {
	normalized := NormalizeSensitiveKey(key)
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
	for _, suffix := range []string{"pwd", "ak", "sk", "api_key", "access_key", "access_key_id", "secret_key", "private_key", "connection_string", "apikey", "accesskey", "accesskeyid", "secretkey", "privatekey", "connectionstring"} {
		if normalized == suffix || strings.HasSuffix(normalized, "_"+suffix) {
			return true
		}
	}
	return false
}

// NormalizeSensitiveKey handles separators, camel case and uppercase acronyms.
func NormalizeSensitiveKey(key string) string {
	if key == "" {
		return ""
	}
	alreadyNormalized := true
	for i := 0; i < len(key); i++ {
		current := key[i]
		if (current >= 'a' && current <= 'z') || (current >= '0' && current <= '9') {
			continue
		}
		if current != '_' || i == 0 || i+1 == len(key) || key[i-1] == '_' {
			alreadyNormalized = false
			break
		}
	}
	if alreadyNormalized {
		return key
	}

	var builder strings.Builder
	builder.Grow(len(key) + 4)
	lastSeparator := true
	for i := 0; i < len(key); i++ {
		current := key[i]
		if !(current >= 'a' && current <= 'z' || current >= 'A' && current <= 'Z' || current >= '0' && current <= '9') {
			if !lastSeparator && builder.Len() > 0 {
				builder.WriteByte('_')
				lastSeparator = true
			}
			continue
		}
		if current >= 'A' && current <= 'Z' {
			previousIsLowerOrDigit := i > 0 && (key[i-1] >= 'a' && key[i-1] <= 'z' || key[i-1] >= '0' && key[i-1] <= '9')
			nextIsLower := i+1 < len(key) && key[i+1] >= 'a' && key[i+1] <= 'z'
			previousIsUpper := i > 0 && key[i-1] >= 'A' && key[i-1] <= 'Z'
			if !lastSeparator && (previousIsLowerOrDigit || previousIsUpper && nextIsLower) {
				builder.WriteByte('_')
			}
			current += 'a' - 'A'
		}
		builder.WriteByte(current)
		lastSeparator = false
	}
	return strings.TrimSuffix(builder.String(), "_")
}
