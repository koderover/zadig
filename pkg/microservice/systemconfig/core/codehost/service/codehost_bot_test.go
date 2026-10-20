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

package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	internalhandler "github.com/koderover/zadig/v2/pkg/shared/handler"
)

func TestCodeHostBotPreserveClearAndReject(t *testing.T) {
	for _, tc := range []struct {
		name, body, wantError string
		cleared               bool
	}{
		{name: "omitted", body: `{}`},
		{name: "explicit null", body: `{"bot":null}`, cleared: true},
		{name: "empty object", body: `{"bot":{}}`, wantError: "gitlab"},
		{name: "wrong provider", body: `{"bot":{"github":{"app_id":2}}}`, wantError: "gitlab"},
		{name: "both providers", body: `{"bot":{"gitlab":{},"github":{}}}`, wantError: "gitlab"},
		{name: "invalid shape", body: `{"bot":"secret"}`, wantError: "bot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := &models.CodeHostBot{GitLab: &models.CodeHostGitLabBot{Token: "old-token"}}
			host := &models.CodeHost{IntegrationLevel: setting.IntegrationLevelSystem, Type: setting.SourceFromGitlab, Bot: old}
			var args CodeHostArgs
			if err := json.Unmarshal([]byte(tc.body), &args); err != nil {
				t.Fatal(err)
			}
			err := applyCodeHostBot(host, args.Bot, "admin")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("got %v, want %q", err, tc.wantError)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if tc.cleared {
				if host.Bot != nil {
					t.Fatal("bot was not cleared")
				}
			} else if host.Bot != old {
				t.Fatal("existing configuration changed")
			}
		})
	}
}

func TestCodeHostGitLabBotValidatesEffectiveSettings(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/" {
			return
		}
		if r.URL.Path != "/api/v4/user" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("PRIVATE-TOKEN") != "service-account-pat" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"username":"ai-review-service-account"}`)
	}))
	defer server.Close()
	for _, tc := range []struct {
		name, raw, wantError string
		existing             bool
	}{
		{name: "first configuration", raw: `{"gitlab":{"token":"service-account-pat"}}`},
		{name: "retained token", raw: `{"gitlab":{}}`, existing: true},
		{name: "blank retains token", raw: `{"gitlab":{"token":""}}`, existing: true},
		{name: "missing first token", raw: `{"gitlab":{}}`, wantError: "首次配置"},
		{name: "invalid token", raw: `{"gitlab":{"token":"bad-token"}}`, existing: true, wantError: "校验失败"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := &models.CodeHost{IntegrationLevel: setting.IntegrationLevelSystem, Type: setting.SourceFromGitlab, Address: server.URL, DisableSSL: true, UpdatedAt: 123}
			if tc.existing {
				host.Bot = &models.CodeHostBot{GitLab: &models.CodeHostGitLabBot{Token: "service-account-pat"}}
			}
			old := host.Bot
			err := applyCodeHostBot(host, json.RawMessage(tc.raw), "admin")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("got %v, want %q", err, tc.wantError)
				}
				if host.Bot != old {
					t.Fatal("failed validation modified stored configuration")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if host.Bot.GitLab.Token != "service-account-pat" || host.Bot.GitLab.BotUsername != "ai-review-service-account" {
				t.Fatal("wrong bot identity")
			}
			if host.Bot.UpdatedBy != "admin" || host.Bot.UpdatedAt == 0 {
				t.Fatal("missing bot audit fields")
			}
			if host.UpdatedAt != 123 {
				t.Fatal("bot update changed OAuth timestamp")
			}
		})
	}
}

func TestCodeHostBotResponseHidesCredentials(t *testing.T) {
	for _, source := range []string{setting.SourceFromGitlab, setting.SourceFromGithub, setting.SourceFromGitee} {
		for _, level := range []setting.IntegrationLevel{setting.IntegrationLevelSystem, setting.IntegrationLevelProject} {
			host := &models.CodeHost{ID: 7, Type: source, IntegrationLevel: level, Alias: "existing-alias", Bot: &models.CodeHostBot{
				GitLab:    &models.CodeHostGitLabBot{Token: "secret-token", BotUsername: "review-bot"},
				GitHub:    &models.CodeHostGitHubBot{AppID: 7, PrivateKey: "secret-key", BotName: "review[bot]"},
				UpdatedBy: "admin", UpdatedAt: 123,
			}}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			internalhandler.JSONResponse(ctx, &internalhandler.Context{Resp: toCodeHostResp(host)})
			response, _ := ctx.Get(setting.ResponseData)
			data, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "secret-token") || strings.Contains(string(data), "secret-key") {
				t.Fatal("response leaked credentials")
			}
			var result map[string]json.RawMessage
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			wantBot := level == setting.IntegrationLevelSystem && source != setting.SourceFromGitee
			if (result["bot"] != nil) != wantBot {
				t.Fatalf("bot presence differs for %s/%s: %s", source, level, data)
			}
			if string(result["alias"]) != `"existing-alias"` || string(result["id"]) != "7" {
				t.Fatal("original codehost fields missing")
			}
			if wantBot {
				var bot map[string]json.RawMessage
				if err := json.Unmarshal(result["bot"], &bot); err != nil {
					t.Fatal(err)
				}
				if string(bot["configured"]) != "true" || bot["codehost_id"] != nil || bot["type"] != nil {
					t.Fatalf("incorrect bot response: %s", result["bot"])
				}
				if source == setting.SourceFromGitlab && bot["github"] != nil {
					t.Fatal("GitLab response included GitHub identity")
				}
				if source == setting.SourceFromGithub && bot["gitlab"] != nil {
					t.Fatal("GitHub response included GitLab identity")
				}
			}
		}
	}
	empty := toCodeHostResp(&models.CodeHost{IntegrationLevel: setting.IntegrationLevelSystem, Type: setting.SourceFromGitlab})
	data, err := json.Marshal(empty.Bot)
	if err != nil || string(data) != `{"configured":false}` {
		t.Fatalf("unconfigured bot: %s, %v", data, err)
	}
}

func TestCodeHostGitHubBotRejectsCredentialChanges(t *testing.T) {
	for _, tc := range []struct{ name, raw, want string }{
		{"zero app ID", `{"github":{"app_id":0}}`, "app_id 必须为正整数"},
		{"negative app ID", `{"github":{"app_id":-1}}`, "app_id 必须为正整数"},
		{"changed app needs key", `{"github":{"app_id":2}}`, "变更 app_id 时需提供对应的 private_key"},
		{"invalid retained key", `{"github":{}}`, "private_key 不是合法的 PEM 格式私钥"},
		{"invalid new key", `{"github":{"private_key":"invalid-new-key"}}`, "private_key 不是合法的 PEM 格式私钥"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := &models.CodeHostBot{GitHub: &models.CodeHostGitHubBot{AppID: 1, PrivateKey: "invalid-existing-key"}}
			host := &models.CodeHost{IntegrationLevel: setting.IntegrationLevelSystem, Type: setting.SourceFromGithub, Bot: old}
			err := applyCodeHostBot(host, json.RawMessage(tc.raw), "admin")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %q", err, tc.want)
			}
			if host.Bot != old || old.GitHub.AppID != 1 || old.GitHub.PrivateKey != "invalid-existing-key" {
				t.Fatal("failed validation changed existing configuration")
			}
		})
	}
}

func TestCodeHostBotRejectsUnsupportedScope(t *testing.T) {
	for _, host := range []*models.CodeHost{
		{IntegrationLevel: setting.IntegrationLevelProject, Type: setting.SourceFromGitlab},
		{IntegrationLevel: setting.IntegrationLevelSystem, Type: setting.SourceFromGitee},
	} {
		if err := applyCodeHostBot(host, json.RawMessage(`{"gitlab":{"token":"pat"}}`), "admin"); err == nil {
			t.Fatal("unsupported scope accepted a bot")
		}
		if err := applyCodeHostBot(host, nil, ""); err != nil {
			t.Fatalf("existing update without bot rejected: %v", err)
		}
	}
}

func TestCreateCodeHostRejectsInvalidBotBeforeWriting(t *testing.T) {
	args := &CodeHostArgs{CodeHost: models.CodeHost{Type: setting.SourceFromGitlab}, Bot: json.RawMessage(`{"gitlab":{}}`)}
	_, err := CreateSystemCodeHost(args, "admin", nil)
	if err == nil || !strings.Contains(err.Error(), "首次配置需提供 token") {
		t.Fatalf("got %v", err)
	}
}
