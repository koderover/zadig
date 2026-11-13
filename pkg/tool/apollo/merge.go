/*
 * Copyright 2026 The KodeRover Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package apollo

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// MergeConfig preserves the original text while applying the planned changes to
// the current Apollo value. Conflict markers are never returned for publishing.
func MergeConfig(ctx context.Context, format, original, planned, current string) (string, error) {
	merged := planned
	switch {
	case planned == original || planned == current:
		merged = current
	case current != original:
		dir, err := os.MkdirTemp("", "apollo-merge-")
		if err != nil {
			return "", fmt.Errorf("create merge directory: %w", err)
		}
		defer os.RemoveAll(dir)
		files := []string{filepath.Join(dir, "planned"), filepath.Join(dir, "original"), filepath.Join(dir, "current")}
		for i, content := range []string{planned, original, current} {
			if err := os.WriteFile(files[i], []byte(content), 0600); err != nil {
				return "", fmt.Errorf("write merge input: %w", err)
			}
		}
		mergeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		output, err := exec.CommandContext(mergeCtx, "git", "merge-file", "--stdout", "--diff3", "--", files[0], files[1], files[2]).Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 && exitErr.ExitCode() < 128 {
				return "", errors.New("configuration merge conflict; refresh the original configuration and edit again")
			}
			return "", fmt.Errorf("git configuration merge failed: %w", err)
		}
		merged = string(output)
	}
	if err := validateConfig(format, merged); err != nil {
		return "", fmt.Errorf("merged configuration is invalid; refresh the original configuration and edit again: %w", err)
	}
	return merged, nil
}

func validateConfig(format, content string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case FormatJSON:
		if !json.Valid([]byte(content)) {
			return errors.New("invalid JSON")
		}
		// YAML's decoder also rejects duplicate object keys in JSON input.
		fallthrough
	case FormatYAML, FormatYML:
		decoder := yaml.NewDecoder(strings.NewReader(content))
		for {
			var value interface{}
			if err := decoder.Decode(&value); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	case FormatXML:
		decoder := xml.NewDecoder(strings.NewReader(content))
		depth, roots := 0, 0
		for {
			token, err := decoder.Token()
			if err == io.EOF {
				if roots != 1 || depth != 0 {
					return errors.New("XML must have exactly one root element")
				}
				return nil
			}
			if err != nil {
				return err
			}
			switch token := token.(type) {
			case xml.StartElement:
				if depth == 0 {
					roots++
				}
				depth++
				attrs := make(map[xml.Name]struct{}, len(token.Attr))
				for _, attr := range token.Attr {
					if _, exists := attrs[attr.Name]; exists {
						return fmt.Errorf("duplicate XML attribute %s", attr.Name.Local)
					}
					attrs[attr.Name] = struct{}{}
				}
			case xml.EndElement:
				depth--
			case xml.CharData:
				if depth == 0 && strings.TrimSpace(string(token)) != "" {
					return errors.New("XML text outside root element")
				}
			}
		}
	}
	return nil
}
