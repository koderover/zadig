package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
)

const allUsers = "*"

type roleBinding struct {
	*policy.RoleBinding
	Username     string `json:"username"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	IdentityType string `json:"identity_type"`
	Account      string `json:"account"`
}

func ListRoleBindings(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]*roleBinding, error) {
	rbs, err := policy.New().ListRoleBindings(header, qs)
	if err != nil {
		logger.Errorf("Failed to list rolebindings, err: %s", err)
		return nil, err
	}

	var uids []string
	uidToRoleBinding := make(map[string][]*roleBinding)
	for _, rb := range rbs {
		if rb.UID != allUsers {
			uids = append(uids, rb.UID)
		}
		uidToRoleBinding[rb.UID] = append(uidToRoleBinding[rb.UID], &roleBinding{RoleBinding: rb})
	}

	users, err := user.New().ListUsers(&user.SearchArgs{UIDs: uids})
	if err != nil {
		logger.Errorf("Failed to list users, err: %s", err)
		return nil, err
	}

	var res []*roleBinding
	for _, u := range users {
		if rb, ok := uidToRoleBinding[u.UID]; ok {
			for _, r := range rb {
				r.Username = u.Name
				r.Email = u.Email
				r.Phone = u.Phone
				r.IdentityType = u.IdentityType
				r.Account = u.Account

				res = append(res, r)
			}

		}
	}

	// add all 'allUsers' roles
	res = append(res, uidToRoleBinding[allUsers]...)

	return res, nil
}
