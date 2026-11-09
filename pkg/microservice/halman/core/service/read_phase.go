package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type readPhaseKey struct{}

type readKey struct{ authorization, path, query string }

type readPhase struct {
	responses map[readKey]json.RawMessage
	bytes     int
}

// 只给一次预览或确认检查合并读取，不能传入异步提交阶段。
func withReadPhase(ctx context.Context) context.Context {
	return context.WithValue(ctx, readPhaseKey{}, &readPhase{responses: map[readKey]json.RawMessage{}})
}

func phaseReads(ctx context.Context, method, path string) *readPhase {
	if method != http.MethodGet {
		return nil
	}
	parts := strings.Split(path, "/")
	template := len(parts) == 5 && parts[1] == "api" && parts[2] == "template" &&
		(parts[3] == "yaml" || parts[3] == "charts" || parts[3] == "build")
	services := len(parts) == 6 && parts[1] == "api" && parts[2] == "project" && parts[3] == "products" && parts[5] == "services"
	if path != "/api/code/codehost" && !template && !services {
		return nil
	}
	cache, _ := ctx.Value(readPhaseKey{}).(*readPhase)
	return cache
}
