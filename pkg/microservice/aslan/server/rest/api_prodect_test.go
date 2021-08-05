package rest

import (
	"context"
	"github.com/koderover/zadig/pkg/microservice/aslan/core"
	"github.com/steinfletcher/apitest"
	"net/http"
	"testing"
)

func TestGetProjects(t *testing.T) {
	core.Start(context.Background())
	apitest.New().
		Handler(NewEngine()).
		Get("/api/project/products").Header("authorization","X-ROOT-API-KEY 9F11B4E503C7F2B5").
		Expect(t).
		Status(http.StatusOK).
		End()
}