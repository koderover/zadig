package service

import (
	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"

	"net/http"
	"net/url"

	"go.uber.org/zap"
)

func CreateWorkflowTask(header http.Header, qs url.Values, body []byte, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().CreateWorkflowTask(header, qs, body)
}

func CancelWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().CancelWorkflowTask(header, qs, id, name)
}

func RestartWorkflowTask(header http.Header, qs url.Values, id string, name string, _ *zap.SugaredLogger) (int, error) {
	return aslan.New().RestartWorkflowTask(header, qs, id, name)
}

func ListWorkflowTask(header http.Header, qs url.Values, commitId string, _ *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().ListWorkflowTask(header, qs, commitId)
}
