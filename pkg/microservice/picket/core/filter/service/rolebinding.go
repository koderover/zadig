package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
)

type roleBinding struct {
	*policy.RoleBinding
	Username string `json:"username"`
}

func ListRoleBindings(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]*roleBinding, error) {
	rbs, err := policy.New().ListRoleBindings(header, qs)
	if err != nil {
		logger.Errorf("Failed to list rolebindings, err: %s", err)
		return nil, err
	}

	var uids []string
	uidToRoleBinding := make(map[string]*roleBinding)
	for _, rb := range rbs {
		uids = append(uids, rb.UID)
		uidToRoleBinding[rb.UID] = &roleBinding{RoleBinding: rb}
	}

	users, err := user.New().ListUsers(&user.SearchArgs{UIDs: uids})
	if err != nil {
		logger.Errorf("Failed to list users, err: %s", err)
		return nil, err
	}

	var res []*roleBinding
	for _, u := range users {
		if rb, ok := uidToRoleBinding[u.UID]; ok {
			rb.Username = u.Name
			res = append(res, rb)
		}
	}

	return res, nil
}
