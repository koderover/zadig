package service

import (
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"go.uber.org/zap"
)

func ListTestWorkflows(testName string, header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	names, err := getAllowedProjects(header, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range names {
		qs.Add("projects", name)
	}

	aslanClient := aslan.New()

	return aslanClient.ListTestProjects(testName, header, qs)
}
