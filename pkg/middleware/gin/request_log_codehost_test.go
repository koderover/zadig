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

package gin

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	gingonic "github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestCodeHostCredentialsAreNotLogged(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		hidden       bool
	}{
		{"POST", "/api/v1/codehosts", true},
		{"PATCH", "/api/v1/codehosts/7", true},
		{"POST", "/api/directory/codehosts", true},
		{"PATCH", "/api/directory/codehosts/7", true},
		{"POST", "/api/v1/other", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			r := gingonic.New()
			r.Use(RequestLog(zap.New(core)))
			body := `{"bot":{"gitlab":{"token":"secret-token"}}}`
			r.Handle(tc.method, tc.path, func(c *gingonic.Context) {
				got, err := io.ReadAll(c.Request.Body)
				if err != nil || string(got) != body {
					t.Fatal("middleware changed the request body")
				}
				c.Status(200)
			})
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, strings.NewReader(body)))
			if logs.Len() != 1 {
				t.Fatalf("got %d log entries", logs.Len())
			}
			got := logs.All()[0].ContextMap()["body"]
			if tc.hidden && got != "" {
				t.Fatal("codehost credentials were captured in request logs")
			}
			if !tc.hidden && got != body {
				t.Fatal("unrelated request logging changed")
			}
		})
	}
}
