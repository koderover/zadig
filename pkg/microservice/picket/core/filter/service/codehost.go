package service

import (
	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/shared/client/systemconfig"
)

func ListCodeHost(_ *zap.SugaredLogger) ([]*systemconfig.CodeHost, error) {
	list, err := systemconfig.New().ListCodeHosts()
	if err != nil {
		return nil, err
	}
	for k, _ := range list {
		list[k].AccessKey = "***"
		list[k].SecretKey = "***"
		list[k].AccessToken = "***"
	}
	return list, nil
}
