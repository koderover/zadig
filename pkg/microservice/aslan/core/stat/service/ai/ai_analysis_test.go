package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/koderover/zadig/v2/pkg/tool/llm"
	"go.uber.org/zap"
)

func TestAnalyzeProjectCompletionBudgetAndTruncation(t *testing.T) {
	for _, truncated := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "truncated"}[truncated], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					MaxTokens int `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.MaxTokens != 16384 {
					t.Errorf("unexpected completion budget: %+v, %v", request, err)
				}
				reason := "end_turn"
				if truncated {
					reason = "max_tokens"
				}
				json.NewEncoder(w).Encode(map[string]any{"content": []map[string]string{{"type": "text", "text": "report"}}, "stop_reason": reason})
			}))
			defer server.Close()
			client, err := llm.NewClientByProtocol(llm.ProtocolAnthropic)
			if err != nil {
				t.Fatal(err)
			}
			if err := client.Configure(llm.LLMConfig{BaseURL: server.URL, Model: "test"}); err != nil {
				t.Fatal(err)
			}
			answer := &analysisAnswer{answer: map[string]string{}, m: &sync.Mutex{}}
			err = AnalyzeProject("分析项目", &ProjectData{ProjectName: "p"}, client, answer, zap.NewNop().Sugar())
			if truncated {
				if !errors.Is(err, llm.ErrMaxTokensExceeded) || len(answer.answer) != 0 {
					t.Fatalf("truncated report must fail without saving a partial answer: %v, %+v", err, answer.answer)
				}
			} else if err != nil || answer.answer["p"] != "report" {
				t.Fatalf("complete report must be retained: %v, %+v", err, answer.answer)
			}
		})
	}
}
