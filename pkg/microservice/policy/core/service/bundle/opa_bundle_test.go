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

package bundle

import (
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/koderover/zadig/pkg/microservice/policy/core/repository/models"
	_ "github.com/koderover/zadig/pkg/util/testing"
)

var testRole1 = `
{
    "name": "author",
    "namespace": "",
    "rules": [
        {
            "methods": ["*"],
            "endpoints": ["/authors"]
        }
    ]
}
`

var testRole2 = `
{
    "name": "superuser",
    "namespace": "project1",
    "rules": [
        {
            "methods": ["GET", "POST"],
            "endpoints": ["/authors", "/articles"]
        }
    ]
}
`
var testBinding1 = `
{
    "name": "b1",
    "namespace": "project1",
    "subjects": [
        {
            "kind": "user",
            "name": "alice"
        },
        {
            "kind": "user",
            "name": "bob"
        }
    ],
    "roleRef": {
        "name": "superuser",
        "namespace": "project1"
    }
}
`

var testBinding2 = `
{
    "name": "b2",
    "namespace": "project1",
    "subjects": [
        {
            "kind": "user",
            "name": "alice"
        }
    ],
    "roleRef": {
        "name": "author",
        "namespace": ""
    }
}
`

var expectOPAData = `
{
    "roles": [
        {
            "name": "author",
            "namespace": "",
            "rules": [
                {
                    "method": "GET",
                    "endpoint": "/authors"
                },
                {
                    "method": "POST",
                    "endpoint": "/authors"
                },
                {
                    "method": "PUT",
                    "endpoint": "/authors"
                },
                {
                    "method": "PATCH",
                    "endpoint": "/authors"
                },
                {
                    "method": "DELETE",
                    "endpoint": "/authors"
                }
            ]
        },
        {
            "name": "superuser",
            "namespace": "project1",
            "rules": [
                {
                    "method": "GET",
                    "endpoint": "/authors"
                },
                {
                    "method": "POST",
                    "endpoint": "/authors"
                },
                {
                    "method": "GET",
                    "endpoint": "/articles"
                },
                {
                    "method": "POST",
                    "endpoint": "/articles"
                }
            ]
        }
    ],

    "role_bindings": [
        {
            "user": "alice",
            "role_refs": [
                {
                    "name": "superuser",
                    "namespace": "project1"
                },
                {
                    "name": "author",
                    "namespace": ""
                }
            ]
        },
        {
            "user": "bob",
            "role_refs": [
                {
                    "name": "superuser",
                    "namespace": "project1"
                }
            ]
        }
    ]
}

`

var _ = Describe("Testing generateOPARoles", func() {

	Context("generateOPARoles", func() {

		var testRoles []*models.Role
		var testBindings []*models.RoleBinding

		BeforeEach(func() {
			r1 := &models.Role{}
			err := json.Unmarshal([]byte(testRole1), r1)
			Expect(err).ShouldNot(HaveOccurred())

			r2 := &models.Role{}
			err = json.Unmarshal([]byte(testRole2), r2)
			Expect(err).ShouldNot(HaveOccurred())

			b1 := &models.RoleBinding{}
			err = json.Unmarshal([]byte(testBinding1), b1)
			Expect(err).ShouldNot(HaveOccurred())

			b2 := &models.RoleBinding{}
			err = json.Unmarshal([]byte(testBinding2), b2)
			Expect(err).ShouldNot(HaveOccurred())

			testRoles = []*models.Role{r1, r2}
			testBindings = []*models.RoleBinding{b1, b2}
		})

		It("should work as expected", func() {
		})

	})
})
