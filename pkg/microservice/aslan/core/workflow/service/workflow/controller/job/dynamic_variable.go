/*
Copyright 2025 The KodeRover Authors.

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

package job

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
	"github.com/koderover/zadig/v2/pkg/util"
)

func getJobVariableKey(currentJobName, serviceName, moduleName, key string, getAvailableVars bool) string {
	resp := ""
	if serviceName == "" && moduleName == "" {
		if getAvailableVars {
			resp = fmt.Sprintf("{{.job.%s.%s}}", currentJobName, key)
		} else {
			resp = fmt.Sprintf("job_%s_%s", currentJobName, key)
		}
	} else {
		if getAvailableVars {
			resp = fmt.Sprintf("{{.job.%s.%s.%s.%s}}", currentJobName, serviceName, moduleName, key)
		} else {
			resp = fmt.Sprintf("job_%s_%s_%s_%s", currentJobName, serviceName, moduleName, key)
		}
	}

	return resp
}

func renderScriptedVariableOptions(ctx *internalhandler.Context, serviceName, moduleName, script, callFunction string, userInput map[string]string) ([]string, error) {
	if script == "" || callFunction == "" {
		return []string{}, nil
	}
	ctx.Logger.Debugf("user input: %+v", userInput)

	callFunction = strings.ReplaceAll(callFunction, "<SERVICE>", serviceName)
	callFunction = strings.ReplaceAll(callFunction, "<MODULE>", moduleName)
	callFunction = replaceJobVariables(callFunction)

	ctx.Logger.Debugf("call function: %s", callFunction)

	t, err := template.New("scriptRender").Parse(callFunction)
	if err != nil {
		return nil, fmt.Errorf("解析调用函数失败, 错误: %v", err)
	}
	t.Option("missingkey=error")

	for key, val := range userInput {
		userInput[key] = "`" + val + "`"
	}

	var realCallFunc bytes.Buffer
	err = t.Execute(&realCallFunc, userInput)
	if err != nil {
		return nil, fmt.Errorf("渲染调用函数失败, 错误: %v", err)
	}

	resp, err := util.RunScriptWithCallFunc(script, realCallFunc.String())
	if err != nil {
		return nil, fmt.Errorf("运行脚本失败， 错误: %v", err)
	}

	return resp, nil
}

func replaceJobVariables(input string) string {
	// 定义正则表达式来匹配 {{.job*}}
	re := regexp.MustCompile(util.GoTemplateKeyRegExp)

	// 使用正则表达式查找所有匹配项
	matches := re.FindAllString(input, -1)

	// 遍历所有匹配项并替换 . 为 _
	for _, match := range matches {
		trimmed := strings.TrimPrefix(match, "{{.")
		trimmed = strings.TrimSuffix(trimmed, "}}")

		replaced := strings.Replace(trimmed, ".", "_", -1)
		replaced = strings.Replace(replaced, "-", "_", -1)

		replaced = "{{." + replaced + "}}"

		// 替换原始字符串中的匹配项
		input = strings.Replace(input, match, replaced, -1)
	}

	return input
}
