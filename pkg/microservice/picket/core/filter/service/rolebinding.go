/*
Copyright 2021 The KodeRover Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package service

import (
	"net/http"
	"net/url"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/koderover/zadig/pkg/microservice/picket/client/policy"
	"github.com/koderover/zadig/pkg/shared/client/user"
)

const ALLUsers = "*"

type roleBinding struct {
	*policy.RoleBinding
	Username     string `json:"username"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	IdentityType string `json:"identity_type"`
	Account      string `json:"account"`
}

type policyBinding struct {
	*policy.PolicyBinding
	Username     string `json:"username"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	IdentityType string `json:"identity_type"`
	Account      string `json:"account"`
}

type Binding struct {
	Roles        []*roleBinding   `json:"roles"`
	Policies     []*policyBinding `json:"policies"`
	UserName     string           `json:"username"`
	Email        string           `json:"email"`
	Account      string           `json:"account"`
	IdentityType string           `json:"identity_type"`
	Phone        string           `json:"phone"`
	Uid          string           `json:"uid"`
}

func ListBindings(header http.Header, qs url.Values, logger *zap.SugaredLogger) ([]*Binding, error) {
	rbs, err := policy.New().ListRoleBindings(header, qs)
	if err != nil {
		logger.Errorf("Failed to list rolebindings, err: %s", err)
		return nil, err
	}

	uidSets := sets.String{}
	uidToRoleBinding := make(map[string][]*roleBinding)
	for _, rb := range rbs {
		if rb.UID != ALLUsers {
			uidSets.Insert(rb.UID)
		}
		uidToRoleBinding[rb.UID] = append(uidToRoleBinding[rb.UID], &roleBinding{RoleBinding: rb})
	}

	pbs, err := policy.New().ListPolicyBindings(header, qs)
	if err != nil {
		logger.Errorf("Failed to list policybindings, err: %s", err)
		return nil, err
	}

	uidToPolicyBindings := make(map[string][]*policyBinding)
	for _, pb := range pbs {
		if pb.UID != ALLUsers {
			uidSets.Insert(pb.UID)
		}
		uidToPolicyBindings[pb.UID] = append(uidToPolicyBindings[pb.UID], &policyBinding{PolicyBinding: pb})
	}
	if uidSets.Len() == 0 {
		// add all 'ALLUsers' roles
		AllUserBinding := &Binding{
			Roles:    uidToRoleBinding[ALLUsers],
			Policies: uidToPolicyBindings[ALLUsers],
			UserName: "*",
			Email:    "",
			Uid:      "*",
		}
		return []*Binding{AllUserBinding}, nil
	}
	users, err := user.New().ListUsers(&user.SearchArgs{UIDs: uidSets.List()})
	if err != nil {
		logger.Errorf("Failed to list users, err: %s", err)
		return nil, err
	}

	var res []*Binding

	var policyBindings []*policyBinding
	for _, u := range users {
		var roleBindings []*roleBinding
		binding := &Binding{
			UserName:     u.Name,
			Email:        u.Email,
			IdentityType: u.IdentityType,
			Account:      u.Account,
			Phone:        u.Phone,
			Uid:          u.UID,
		}
		if rb, ok := uidToRoleBinding[u.UID]; ok {
			for _, r := range rb {
				r.Username = u.Name
				r.Email = u.Email
				r.Phone = u.Phone
				r.IdentityType = u.IdentityType
				r.Account = u.Account
				roleBindings = append(roleBindings, r)
			}
			binding.Roles = roleBindings
		} else {
			logger.Warnf("can not find user's role bindings , uid:%v", u.UID)
		}
		if pb, ok := uidToPolicyBindings[u.UID]; ok {
			for _, p := range pb {
				p.Username = u.Name
				p.Email = u.Email
				p.Phone = u.Phone
				p.IdentityType = u.IdentityType
				p.Account = u.Account
				policyBindings = append(policyBindings, p)
			}
			binding.Policies = policyBindings
		} else {
			logger.Warnf("can not find user's policy bindings , uid:%v", u.UID)
		}
		res = append(res, binding)
	}

	// add all 'ALLUsers' roles
	AllUserBinding := &Binding{
		Roles:    uidToRoleBinding[ALLUsers],
		Policies: uidToPolicyBindings[ALLUsers],
		UserName: "*",
		Email:    "",
		Uid:      "*",
	}
	res = append(res, AllUserBinding)
	return res, nil
}
