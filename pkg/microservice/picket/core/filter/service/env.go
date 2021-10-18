package service

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/koderover/zadig/pkg/microservice/picket/client/opa"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func ListEnvs(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	allow, err := opaAllowEnvList(header, logger)
	if err != nil {
		logger.Errorf("Failed to get allowed project names, err: %s", err)
		return nil, err
	}
	if !allow {
		return nil, errors.New("have no list env permission")
	}
	aslanClient := aslan.New()

	return aslanClient.ListEnvs(header, qs)
}

func opaAllowEnvList(headers http.Header, logger *zap.SugaredLogger) (bool, error) {
	res := &opaAllowResp{}

	opaClient := opa.NewDefault()
	err := opaClient.Evaluate("rbac.allow", res, func() (*opa.Input, error) {
		return generateOPAInput(headers, "GET", "api/aslan/environment/enviromments"), nil
	})
	if err != nil {
		logger.Errorf("opa evaluation failed, err: %s", err)
		return false, err
	}

	return res.Result, nil
}

type opaAllowResp struct {
	Result bool `json:"result"`
}
