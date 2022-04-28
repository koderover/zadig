package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/aslan"
)

func Overview(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Overview(header, qs)
}

func Test(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Test(header, qs)
}

func Deploy(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Deploy(header, qs)
}

func Build(header http.Header, qs url.Values, log *zap.SugaredLogger) ([]byte, error) {
	return aslan.New().Build(header, qs)
}
