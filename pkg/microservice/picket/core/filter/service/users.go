package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/microservice/picket/client/user"
)

type DeleteUserResp struct{
	Message string `json:"message"`
}

func DeleteUser(userID string ,header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]byte, error) {
	_ , err := user.New().DeleteUser(userID,header,qs)
	if err !=nil {
		return []byte{}, err
	}
	return  policy.New().DeleteRoleBindings(userID,header,qs)
}
