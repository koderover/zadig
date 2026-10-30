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

package apisix

import (
	"strings"
	"text/scanner"
)

type ProtoOptionalField struct {
	Field string `json:"field"`
	Line  int    `json:"line"`
}

// FindProto3OptionalFields scans declarations without resolving imports or
// compiling the proto. It recognizes optional fields, not full syntax validity.
func FindProto3OptionalFields(content string) []ProtoOptionalField {
	type token struct {
		kind rune
		text string
		line int
	}
	var s scanner.Scanner
	s.Init(strings.NewReader(content))
	s.Mode = scanner.ScanIdents | scanner.ScanInts | scanner.ScanStrings | scanner.ScanChars | scanner.ScanComments | scanner.SkipComments
	// Proto permits multi-character single-quoted strings, unlike Go characters.
	s.Error = func(*scanner.Scanner, string) {}
	var tokens []token
	for kind := s.Scan(); kind != scanner.EOF; kind = s.Scan() {
		tokens = append(tokens, token{kind, s.TokenText(), s.Position.Line})
	}
	if len(tokens) < 4 || tokens[0].text != "syntax" || tokens[1].text != "=" ||
		(tokens[2].text != `"proto3"` && tokens[2].text != "'proto3'") || tokens[3].text != ";" {
		return nil
	}

	var fields []ProtoOptionalField
	// Empty scope names represent enum/oneof/option bodies, where optional is
	// not a message field declaration.
	var scopes []string
	var packageName string
	for i := 4; i < len(tokens); i++ {
		t := tokens[i]
		inMessage := len(scopes) > 0 && scopes[len(scopes)-1] != ""
		if t.text == "package" && len(scopes) == 0 {
			for i++; i < len(tokens) && tokens[i].text != ";"; i++ {
				packageName += tokens[i].text
			}
			continue
		}
		if t.text == "message" && (len(scopes) == 0 || inMessage) && i+2 < len(tokens) &&
			tokens[i+1].kind == scanner.Ident && tokens[i+2].text == "{" {
			scopes = append(scopes, tokens[i+1].text)
			i += 2
			continue
		}
		switch t.text {
		case "{":
			scopes = append(scopes, "")
		case "}":
			if len(scopes) > 0 {
				scopes = scopes[:len(scopes)-1]
			}
		case "optional":
			if !inMessage {
				continue
			}
			// optional [.]Type[.Nested] field = number [options];
			j := i + 1
			if j < len(tokens) && tokens[j].text == "." {
				j++
			}
			if j >= len(tokens) || tokens[j].kind != scanner.Ident {
				continue
			}
			j++
			for j+1 < len(tokens) && tokens[j].text == "." && tokens[j+1].kind == scanner.Ident {
				j += 2
			}
			if j+3 >= len(tokens) || tokens[j].kind != scanner.Ident || tokens[j+1].text != "=" ||
				tokens[j+2].kind != scanner.Int || (tokens[j+3].text != ";" && tokens[j+3].text != "[") {
				continue
			}
			path := strings.Join(scopes, ".") + "." + tokens[j].text
			if packageName != "" {
				path = packageName + "." + path
			}
			fields = append(fields, ProtoOptionalField{Field: path, Line: t.line})
		}
	}
	return fields
}
