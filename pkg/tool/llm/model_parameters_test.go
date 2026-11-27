package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/koderover/zadig/v2/pkg/tool/log"
)

func TestCompletionModelParameters(t *testing.T) {
	log.Init(&log.Config{Level: "error"})
	for _, tc := range []struct {
		protocol Protocol
		provider Provider
		tokenKey string
		effort   string
		response string
	}{
		{ProtocolOpenAI, ProviderOpenAI, "max_completion_tokens", `"reasoning_effort":"high"`, `{"choices":[{"message":{"content":"ok"}}]}`},
		{ProtocolOpenAI, ProviderOther, "max_tokens", `"reasoning_effort":"high"`, `{"choices":[{"message":{"content":"ok"}}]}`},
		{ProtocolAnthropic, ProviderOther, "max_tokens", `"output_config":{"effort":"high"}`, `{"content":[{"type":"text","text":"ok"}]}`},
	} {
		t.Run(string(tc.protocol)+"/"+string(tc.provider), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				var payload map[string]json.RawMessage
				if err != nil || json.Unmarshal(body, &payload) != nil ||
					string(payload[tc.tokenKey]) != "64" || !strings.Contains(string(body), tc.effort) {
					t.Errorf("unexpected request: %s", body)
				}
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, tc.response)
			}))
			defer server.Close()

			client, err := NewClientByProtocol(tc.protocol)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Configure(LLMConfig{
				ProviderName: tc.provider, Model: "test", BaseURL: server.URL,
				ModelParameters: ModelParameters{ContextWindow: 256, MaxOutputTokens: 128, ReasoningEffort: "high"},
			}); err != nil {
				t.Fatal(err)
			}
			if result, err := client.GetCompletion(context.Background(), "hello", WithMaxTokens(64)); err != nil || result != "ok" {
				t.Fatalf("GetCompletion() = %q, %v", result, err)
			}
			if _, err := client.GetCompletion(context.Background(), strings.Repeat("hello ", 200)); err == nil {
				t.Fatal("expected an error for a prompt exceeding the configured context window")
			}
		})
	}
}

func TestModelParametersValidation(t *testing.T) {
	for _, tc := range []struct {
		parameters ModelParameters
		protocol   Protocol
		valid      bool
	}{
		{ModelParameters{}, ProtocolOpenAI, true},
		{ModelParameters{ContextWindow: 128}, ProtocolOpenAI, false},
		{ModelParameters{ContextWindow: 128, MaxOutputTokens: 128}, ProtocolOpenAI, false},
		{ModelParameters{ContextWindow: 128, MaxOutputTokens: 127, ReasoningEffort: "none"}, ProtocolOpenAI, true},
		{ModelParameters{ReasoningEffort: "minimal"}, ProtocolAnthropic, false},
		{ModelParameters{ReasoningEffort: "xhigh"}, ProtocolAnthropic, true},
	} {
		if err := tc.parameters.Validate(tc.protocol); (err == nil) != tc.valid {
			t.Fatalf("Validate(%+v, %s) = %v", tc.parameters, tc.protocol, err)
		}
	}
}
