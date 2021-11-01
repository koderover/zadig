package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"
)

func ListTestWorkflows(testName string, header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	rules := []*rule{{
		method:   "/api/aslan/workflow/workflow",
		endpoint: "PUT",
	}}
	names, err := getAllowedProjectsWithUpdateWorkflowsPermission(header, rules, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	if len(names) == 1 && names[0] == "*" {
		return aslan.New().ListTestWorkflows(testName, header, qs)
	}

	for _, name := range names {
		qs.Add("projects", name)
	}

	return aslan.New().ListTestWorkflows(testName, header, qs)
}

type rule struct {
	method   string
	endpoint string
}

func getAllowedProjectsWithUpdateWorkflowsPermission(headers http.Header, rules []*rule, logger *zap.SugaredLogger) (projects []string, err error) {
	resps := [][]string{}
	for _, v := range rules {
		res := &allowedProjectsData{}
		opaClient := opa.NewDefault()
		err := opaClient.Evaluate("rbac.user_allowed_projects", res, func() (*opa.Input, error) {
			return generateOPAInput(headers, v.method, v.endpoint), nil
		})
		if err != nil {
			logger.Errorf("opa evaluation failed, err: %s", err)
			return nil, err
		}
		resps = append(resps, res.Result)
	}
	return intersect(resps), nil
}

func intersect(s [][]string) []string {
	if len(s) == 0 {
		return nil
	}
	tmp := sets.NewString(s[0]...)
	for _, v := range s[1:] {
		t := sets.NewString(v...)
		tmp = t.Intersection(tmp)
	}
	return tmp.List()
}
