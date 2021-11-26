package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func DownloadKubeConfig(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	readEnvRules := []*rule{{
		method:   "GET",
		endpoint: "/api/aslan/environment/environments",
	}}
	editEnvRules := []*rule{{
		method:   "POST",
		endpoint: "/api/aslan/environment/environments/?*",
	}, {
		method:   "POST",
		endpoint: "/api/aslan/environment/environments/?*/services/?*/restart",
	},
	}
	readEnvProjects, err := getAllowedProjects(header, readEnvRules, true, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range readEnvProjects {
		qs.Add("readEnvProjects", name)
	}

	editEnvProjects, err := getAllowedProjects(header, editEnvRules, true, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range editEnvProjects {
		qs.Add("editEnvProjects", name)
	}

	return aslan.New().DownloadKubeConfig(header, qs)
}
