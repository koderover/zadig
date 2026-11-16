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

	"github.com/bufbuild/protocompile/ast"
	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
)

type ProtoOptionalField struct {
	Field string `json:"field"`
	Line  int    `json:"line"`
}

// FindProto3OptionalFields parses declarations without resolving imports or
// compiling the proto. It recognizes optional fields, not full syntax validity.
func FindProto3OptionalFields(content string) []ProtoOptionalField {
	// Continue after syntax errors and inspect the recovered AST where possible.
	// Syntax validation is outside the scope of this compatibility check.
	handler := reporter.NewHandler(reporter.NewReporter(func(reporter.ErrorWithPos) error { return nil }, nil))
	file, _ := parser.Parse("apisix.proto", strings.NewReader(content), handler)
	if file == nil || file.Syntax == nil || file.Syntax.Syntax.AsString() != "proto3" {
		return nil
	}

	var prefix string
	for _, decl := range file.Decls {
		if pkg, ok := decl.(*ast.PackageNode); ok {
			prefix = string(pkg.Name.AsIdentifier()) + "."
		}
	}

	var fields []ProtoOptionalField
	var visitMessage func(*ast.MessageNode, string)
	visitMessage = func(message *ast.MessageNode, prefix string) {
		path := prefix + message.Name.Val + "."
		for _, decl := range message.Decls {
			switch node := decl.(type) {
			case *ast.MessageNode:
				visitMessage(node, path)
			case *ast.FieldNode:
				if node.Label.IsPresent() && node.Label.Val == "optional" {
					fields = append(fields, ProtoOptionalField{
						Field: path + node.Name.Val,
						Line:  file.NodeInfo(node.Label.KeywordNode).Start().Line,
					})
				}
			}
		}
	}
	for _, decl := range file.Decls {
		if message, ok := decl.(*ast.MessageNode); ok {
			visitMessage(message, prefix)
		}
	}
	return fields
}
