package ai

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"github.com/koderover/zadig/v2/pkg/util"
)

type BuildLogAnalysisArgs struct {
	Log string `json:"log"`
}

func AnalyzeBuildLog(args *BuildLogAnalysisArgs, project, pipeline, job string, taskID int64, logger *zap.SugaredLogger) (string, error) {
	ctx := context.Background()
	client, err := service.GetDefaultLLMClient(ctx)
	if err != nil {
		logger.Errorf("failed to get llm client, the error is: %+v", err)
		return "", err
	}

	log := args.Log
	prompt := fmt.Sprintf("%s; 构建日志数据: \"\"\"%s\"\"\"", BuildLogAnalysisPrompt, util.RemoveExtraSpaces(splitBuildLogByRowNum(log, 500)))

	options := []llm.ParamOption{}
	if client.GetModel() != "" {
		options = append(options, llm.WithModel(client.GetModel()))
	} else {
		options = append(options, llm.WithModel(openapi.GPT4o))
	}
	answer, err := client.GetCompletion(ctx, prompt, options...)
	if err != nil {
		logger.Errorf("failed to get answer from ai: %v, the error is: %+v", client.GetName(), err)
		return "", err
	}

	return answer, nil
}

func calculateTokenNum(msg string) (int, error) {
	num, err := llm.NumTokensFromPrompt(msg, "")
	if err != nil {
		return 0, err
	}
	return num, nil
}

// splitBuildLog TODO: need to be optimized, consider that how to split by build steps
func splitBuildLogByRowNum(log string, num int) string {
	logs := make([]string, 0)
	temp := strings.Split(log, "\n")
	for _, t := range temp {
		trimmedLine := strings.TrimSpace(t)
		if trimmedLine != "" {
			logs = append(logs, trimmedLine)
		}
	}

	result := make([]string, 0)
	start, end := 0, len(logs)-1
	if num < len(logs) {
		start = len(logs) - num
	}
	for i := start; i <= end; i++ {
		result = append(result, logs[i])
	}

	return fmt.Sprintf("构建日志最后 %d 行日志:%s", num, strings.Join(result, ";"))
}

func splitBuildLogByStep(log string) string {
	logs := strings.Split(log, "\n")
	log = ""
	for i := 0; i < len(logs); i++ {
		if strings.Contains(logs[i], "============== Step Start>") {
			log += logs[i] + "\n"
			log += "...省略该步骤的详细日志...\n"
			for j := i; j < len(logs); j++ {
				if strings.Contains(logs[j], "============== Step End>") {
					log += logs[j] + "\n"
					i = j
					break
				}
			}
		}
	}
	return log
}
