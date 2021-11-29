package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	consts "github.com/koderover/zadig/pkg/microservice/picket/core/const"
)

//DownloadKubeConfig user download kube config file which has permission to read or edit namespaces he has permission to
//query the opa service to get the project lists by pass through *rules parameter action
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
	projectsEnvCanView, err := getAllowedProjects(header, readEnvRules, consts.OR, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range projectsEnvCanView {
		qs.Add("projectsEnvCanView", name)
	}

	projectsEnvCanEdit, err := getAllowedProjects(header, editEnvRules, consts.OR, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}

	for _, name := range projectsEnvCanEdit {
		qs.Add("projectsEnvCanEdit", name)
	}

	return aslan.New().DownloadKubeConfig(header, qs)
}
