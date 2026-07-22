package main

import (
	"context"
	"log"
	"net/http"
	"os"

	adkagent "google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/cmd/launcher"
	"google.golang.org/adk/v2/cmd/launcher/full"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"

	"hello-agent/internal/llmbridge"
	"hello-agent/internal/llmconfig"
	"hello-agent/internal/runbook"
)

func main() {
	ctx := context.Background()

	cfg, err := llmconfig.Load()
	if err != nil {
		log.Fatalf("load model config: %v", err)
	}

	rootAgent, err := newDevOpsAgent(cfg)
	if err != nil {
		log.Fatalf("create agent: %v", err)
	}

	l := full.NewLauncher()
	if err := l.Execute(ctx, &launcher.Config{
		AgentLoader: adkagent.NewSingleLoader(rootAgent),
	}, os.Args[1:]); err != nil {
		log.Fatalf("run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

func newDevOpsAgent(cfg llmconfig.Config) (adkagent.Agent, error) {
	runbookTool, err := functiontool.New(
		functiontool.Config{
			Name:        "lookup_runbook",
			Description: "Look up the DevOps runbook for a service and optional incident symptom.",
		},
		func(ctx adkagent.Context, args runbook.Args) (runbook.Result, error) {
			return runbook.Lookup(args)
		},
	)
	if err != nil {
		return nil, err
	}

	return llmagent.New(llmagent.Config{
		Name:        "devops_runbook_agent",
		Model:       llmbridge.New(cfg, &http.Client{Timeout: cfg.Timeout}),
		Description: "DevOps assistant that uses runbooks to help triage service incidents.",
		Instruction: `You are a DevOps incident triage assistant.
Use lookup_runbook when the user asks about service incidents, symptoms, owners, metrics, diagnostics, or escalation.
Base operational advice on tool output. If a service is unknown, say so and ask for a known service name.
Keep answers concise, actionable, and ordered by immediate next steps.`,
		Tools: []tool.Tool{runbookTool},
	})
}
