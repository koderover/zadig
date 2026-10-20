/*
Copyright 2026 The KodeRover Authors.

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

package mongodb

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"

	"github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
)

func TestCodeHostUpdatePersistsBotAtomically(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()
	for _, tc := range []struct {
		name             string
		updateBot, clear bool
	}{
		{name: "omitted bot"},
		{name: "configured bot", updateBot: true},
		{name: "clear bot", updateBot: true, clear: true},
	} {
		mt.Run(tc.name, func(mt *mtest.T) {
			host := &models.CodeHost{ID: 7, Type: setting.SourceFromGitlab, Alias: "new-alias", AccessToken: "oauth-token", RefreshToken: "oauth-refresh", UpdatedAt: 123,
				Bot: &models.CodeHostBot{GitLab: &models.CodeHostGitLabBot{Token: "bot-pat"}},
			}
			if tc.clear {
				host.Bot = nil
			}
			mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}))
			coll := &CodehostColl{Collection: mt.Coll}
			if _, err := coll.UpdateCodeHost(host, tc.updateBot); err != nil {
				mt.Fatal(err)
			}
			events := mt.GetAllStartedEvents()
			if len(events) != 1 || events[0].CommandName != "update" {
				mt.Fatal("codehost and bot did not use one update")
			}
			update := events[0].Command.Lookup("updates").Array().Index(0).Value().Document().Lookup("u").Document()
			set := update.Lookup("$set").Document()
			if set.Lookup("alias").StringValue() != "new-alias" || set.Lookup("access_token").StringValue() != "oauth-token" || set.Lookup("refresh_token").StringValue() != "oauth-refresh" || set.Lookup("updated_at").Int64() != 123 {
				mt.Fatal("codehost or OAuth fields were changed incorrectly")
			}
			_, setBotErr := set.LookupErr("bot")
			unset, unsetErr := update.LookupErr("$unset")
			if tc.updateBot && !tc.clear {
				if setBotErr != nil || set.Lookup("bot").Document().Lookup("gitlab").Document().Lookup("token").StringValue() != "bot-pat" {
					mt.Fatal("bot not persisted with codehost")
				}
			} else if setBotErr == nil {
				mt.Fatal("bot was overwritten unexpectedly")
			}
			if tc.clear {
				if unsetErr != nil {
					mt.Fatal("bot not removed")
				}
				if _, err := unset.Document().LookupErr("bot"); err != nil {
					mt.Fatal("bot not removed")
				}
			} else if unsetErr == nil {
				mt.Fatal("unrelated update removed bot")
			}
		})
	}
}
