// Forked from github.com/k8sgpt-ai/k8sgpt
// Some parts of this file have been modified to make it functional in Zadig

package analysis

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	NormalResultOutput string = "没有检测到问题"
)

var outputFormats = map[string]func(*Analysis) ([]byte, error){
	"json": (*Analysis).jsonOutput,
	"text": (*Analysis).textOutput,
}

func getOutputFormats() []string {
	formats := make([]string, 0, len(outputFormats))
	for format := range outputFormats {
		formats = append(formats, format)
	}
	return formats
}

func (a *Analysis) PrintOutput(format string) ([]byte, error) {
	outputFunc, ok := outputFormats[format]
	if !ok {
		return nil, fmt.Errorf("unsupported output format: %s. Available format %s", format, strings.Join(getOutputFormats(), ","))
	}
	return outputFunc(a)
}

func (a *Analysis) jsonOutput() ([]byte, error) {
	var problems int
	var status AnalysisStatus
	for _, result := range a.Results {
		problems += len(result.Error)
	}
	if problems > 0 {
		status = StateProblemDetected
	} else {
		status = StateOK
	}

	result := JsonOutput{
		Provider: a.AnalysisAIProvider,
		Problems: problems,
		Results:  a.Results,
		Errors:   a.Errors,
		Status:   status,
	}
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("error marshalling json: %v", err)
	}
	return output, nil
}

func (a *Analysis) textOutput() ([]byte, error) {
	var output strings.Builder

	// Print the AI provider used for this analysis
	// output.WriteString(fmt.Sprintf("AI Provider: %s\n", a.AnalysisAIProvider))

	if len(a.Errors) != 0 {
		output.WriteString("\n")
		output.WriteString("警告: \n")
		for _, aerror := range a.Errors {
			output.WriteString(fmt.Sprintf("- %s\n", aerror))
		}
	}
	output.WriteString("\n")
	if len(a.Results) == 0 {
		output.WriteString(NormalResultOutput + "\n")
		return []byte(output.String()), nil
	}
	for n, result := range a.Results {
		output.WriteString(fmt.Sprintf("#%s %s(%s)\n", fmt.Sprintf("%d", n), result.Name, result.ParentObject))
		for _, err := range result.Error {
			output.WriteString(fmt.Sprintf("%s %s\n\n", "原始错误:", err.Text))
			// if err.KubernetesDoc != "" {
			// 	output.WriteString(fmt.Sprintf("  %s %s\n", "Kubernetes Doc:", err.KubernetesDoc))
			// }
		}
		output.WriteString(result.Details + "\n\n")
	}
	return []byte(output.String()), nil
}
