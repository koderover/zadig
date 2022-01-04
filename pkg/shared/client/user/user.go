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

package user

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type User struct {
	UID          string `json:"uid"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	IdentityType string `json:"identity_type"`
	Account      string `json:"account"`
}

type usersResp struct {
	Users []*User `json:"users"`
}

type SearchArgs struct {
	UIDs []string `json:"uids"`
}

func (c *Client) ListUsers(args *SearchArgs) ([]*User, error) {
	url := "/users/search"

	res := &usersResp{}
	_, err := c.Post(url, httpclient.SetResult(res), httpclient.SetBody(args))
	if err != nil {
		return nil, err
	}

	return res.Users, err
}

type CreateUserArgs struct {
	Name     string `json:"name"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Account  string `json:"account"`
}

type CreateUserResp struct {
	Name    string `json:"name"`
	Account string `json:"account"`
	Uid     string `json:"uid"`
}

func (c *Client) CreateUser(args *CreateUserArgs) (*CreateUserResp, error) {
	url := "/users"
	resp := &CreateUserResp{}
	_, err := c.Post(url, httpclient.SetBody(args), httpclient.SetResult(resp))
	return resp, err
}

type SearchUserArgs struct {
	Account string `json:"account"`
}

type SearchUserResp struct {
	TotalCount int     `json:"totalCount"`
	Users      []*User `json:"users"`
}

func (c *Client) SearchUser(args *SearchUserArgs) (*SearchUserResp, error) {
	url := "/users/search"
	resp := &SearchUserResp{}
	_, err := c.Post(url, httpclient.SetBody(args), httpclient.SetResult(resp))
	return resp, err
}

func (c *Client) Healthz() error {
	url := "/healthz"
	_, err := c.Get(url)
	return err
}
