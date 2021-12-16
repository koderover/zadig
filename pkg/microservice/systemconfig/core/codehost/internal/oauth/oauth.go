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

package oauth

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

type Oauth interface {
	LoginURL(state string) (loginURL string)
	HandleCallback(r *http.Request) (*oauth2.Token, error)
}

type OAuth2Error struct {
	Err         string
	Description string
}

func (e *OAuth2Error) Error() string {
	if e.Description == "" {
		return e.Err
	}
	return fmt.Sprintf("%s: %s", e.Err, e.Description)
}
