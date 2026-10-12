package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	commonrepo "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	commonservice "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service"
	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/webhook"
	codehostmodels "github.com/koderover/zadig/v2/pkg/microservice/systemconfig/core/codehost/repository/models"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
	"github.com/koderover/zadig/v2/pkg/types"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestAIReviewWebhookUsesSelectedRepository(t *testing.T) {
	for _, source := range []string{types.ProviderGithub, types.ProviderGitlab} {
		for _, ctl := range []*commonmodels.ScanningHookCtl{nil, {Enabled: false}, {Enabled: true}} {
			args := &Scanning{ScannerType: types.ScannerTypeAIReview, Repos: []*types.Repository{{Source: source, CodehostID: 3, RepoNamespace: "org", RepoName: "repo"}}, AdvancedSetting: &commonmodels.ScanningAdvancedSetting{HookCtl: ctl}}
			hooks, err := scanningWebhookItems(args)
			if err != nil || len(hooks) != 1 || hooks[0].CodehostID != 3 || hooks[0].RepoOwner != "org" || hooks[0].RepoName != "repo" {
				t.Fatalf("source=%s hooks=%+v err=%v", source, hooks, err)
			}
		}
	}
}

func TestAIReviewScanningCleanupAfterCodehostDeletion(t *testing.T) {
	uri := os.Getenv("AI_REVIEW_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set AI_REVIEW_TEST_MONGO_URI for local integration test")
	}
	log.Init(&log.Config{Level: "error"})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("ai_review_scanning_cleanup_test_%d", time.Now().UnixNano())
	oldDB := viper.Get(setting.ENVAslanDBName)
	viper.Set(setting.ENVAslanDBName, dbName)
	defer viper.Set(setting.ENVAslanDBName, oldDB)
	mongotool.Init(ctx, uri)
	db := mongotool.Database(dbName)
	defer db.Drop(context.Background())
	stop := make(chan struct{})
	go webhook.NewWebhookController().Run(1, stop)
	defer close(stop)
	for i, provider := range []string{setting.SourceFromGithub, setting.SourceFromGitlab} {
		id := i + 1
		address := "https://git.example"
		owner := "org"
		if provider == setting.SourceFromGitlab {
			owner = "org/group"
		}
		if _, err := db.Collection(codehostmodels.CodeHost{}.TableName()).InsertOne(ctx, codehostmodels.CodeHost{ID: id, Type: provider, Address: address, DeletedAt: time.Now().Unix()}); err != nil {
			t.Fatal(err)
		}
		args := &Scanning{ScannerType: types.ScannerTypeAIReview, Repos: []*types.Repository{{CodehostID: id, RepoOwner: "org", RepoNamespace: owner, RepoName: "repo"}}}
		hooks, err := scanningWebhookItems(args)
		if err != nil || len(hooks) != 1 || hooks[0].Source != provider || hooks[0].RepoOwner != owner {
			t.Fatalf("failed to recover deleted provider: %+v err=%v", hooks, err)
		}
		// New configurations must still reject a deleted codehost.
		if err := commonservice.ProcessWebhook(hooks, nil, "scanning-invalid", log.SugaredLogger(), true); err == nil {
			t.Fatal("new configuration accepted a deleted codehost")
		}
		module := &commonmodels.Scanning{ID: primitive.NewObjectID(), Name: fmt.Sprint("legacy-", id), ScannerType: types.ScannerTypeAIReview, Repos: args.Repos}
		if err := commonrepo.NewScanningColl().Create(module); err != nil {
			t.Fatal(err)
		}
		_, err = db.Collection(commonmodels.WebHook{}.TableName()).InsertOne(ctx, commonmodels.WebHook{Owner: owner, Repo: "repo", Address: address, References: []string{"scanning-" + module.Name + "-repo"}})
		if err != nil {
			t.Fatal(err)
		}
		if err := DeleteScanningModuleByID(module.ID.Hex(), log.SugaredLogger()); err != nil {
			t.Fatal(err)
		}
		if _, err := commonrepo.NewScanningColl().GetByID(module.ID.Hex()); !errors.Is(err, mongo.ErrNoDocuments) {
			t.Fatalf("scanning module was not deleted: %v", err)
		}
		if _, err := commonrepo.NewWebHookColl().Find(owner, "repo", address); !errors.Is(err, mongo.ErrNoDocuments) {
			t.Fatalf("webhook reference was not removed: %v", err)
		}
	}
}

func TestAIReviewWebhookReusesAutomaticTriggerRepository(t *testing.T) {
	for _, source := range []string{types.ProviderGithub, types.ProviderGitlab} {
		hook := &commonmodels.ScanningHook{CodehostID: 3, RepoOwner: "org", RepoName: "repo", IsManual: true}
		args := &Scanning{ScannerType: types.ScannerTypeAIReview, Repos: []*types.Repository{{Source: source, CodehostID: 3, RepoNamespace: "org", RepoName: "repo"}}, AdvancedSetting: &commonmodels.ScanningAdvancedSetting{HookCtl: &commonmodels.ScanningHookCtl{Enabled: true, Items: []*commonmodels.ScanningHook{hook}}}}
		hooks, err := scanningWebhookItems(args)
		if err != nil || len(hooks) != 1 || hooks[0] != hook {
			t.Fatalf("existing trigger duplicated or replaced: hooks=%+v err=%v", hooks, err)
		}
	}
}

func TestOtherScanningWebhooksUnchanged(t *testing.T) {
	for _, args := range []*Scanning{
		{ScannerType: types.ScannerTypeSonarQube, Repos: []*types.Repository{{Source: types.ProviderGitlab}}},
		{ScannerType: types.ScannerTypeAIReview, Repos: []*types.Repository{{Source: types.ProviderGitee}}},
	} {
		hooks, err := scanningWebhookItems(args)
		if err != nil || len(hooks) != 0 {
			t.Fatalf("unexpected feedback hook: hooks=%+v err=%v", hooks, err)
		}
	}
}
