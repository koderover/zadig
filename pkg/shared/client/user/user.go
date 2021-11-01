package user

import (
	"github.com/koderover/zadig/pkg/tool/httpclient"
)

type User struct {
	UID   string `json:"uid"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
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
	Name     string
	Password string
	Email    string
	Phone    string
	Account  string
}

func (c *Client) CreateUser(args *CreateUserArgs) error {
	url := "/users"
	_, err := c.Post(url, httpclient.SetBody(args))
	return err
}

type SearchUserArgs struct {
	Account string
}

type SearchUserResp struct {
	TotalCount int               `json:"totalCount"`
	Users      []*CreateUserArgs `json:"users"`
}

func (c *Client) SearchUser(args *SearchUserArgs) (*SearchUserResp, error) {
	url := "/users/search"
	resp := &SearchUserResp{}
	_, err := c.Post(url, httpclient.SetBody(args), httpclient.SetBody(resp))
	return resp, err
}
