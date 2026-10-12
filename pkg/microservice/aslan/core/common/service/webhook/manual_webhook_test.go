package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/mongodb"
	"github.com/koderover/zadig/v2/pkg/setting"
	"github.com/koderover/zadig/v2/pkg/tool/log"
	mongotool "github.com/koderover/zadig/v2/pkg/tool/mongo"
)

type manualWebhookTransport func(*http.Request) (*http.Response, error)

func (f manualWebhookTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestManualAIReviewWebhookOnlyMaintainsReferences(t *testing.T) {
	uri := os.Getenv("AI_REVIEW_TEST_MONGO_URI")
	if uri == "" {
		t.Skip("set AI_REVIEW_TEST_MONGO_URI for local integration test")
	}
	log.Init(&log.Config{Level: "error"})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dbName := fmt.Sprintf("ai_review_manual_webhook_test_%d", time.Now().UnixNano())
	oldDB := viper.Get(setting.ENVAslanDBName)
	viper.Set(setting.ENVAslanDBName, dbName)
	defer viper.Set(setting.ENVAslanDBName, oldDB)
	mongotool.Init(ctx, uri)
	defer mongotool.Database(dbName).Drop(context.Background())
	var remoteCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		remoteCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `[]`)
		} else {
			fmt.Fprint(w, `{"id":9,"emoji_events":true}`)
		}
	}))
	defer server.Close()
	oldClient := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: manualWebhookTransport(func(*http.Request) (*http.Response, error) {
		remoteCalls.Add(1)
		return nil, errors.New("manual webhook mode must not contact GitHub")
	})}
	defer func() { http.DefaultClient = oldClient }()
	coll := mongodb.NewWebHookColl()
	for _, provider := range []string{setting.SourceFromGithub, setting.SourceFromGitlab} {
		t.Run(provider, func(t *testing.T) {
			job := &task{owner: "org", repo: provider, address: server.URL, from: provider, isManual: true, ensureAIReviewWebhook: true, add: true, doneCh: make(chan struct{}, 1)}
			for _, ref := range []string{"scanning-first-trigger", "scanning-second-trigger"} {
				job.ref = ref
				addWebhook(job, zap.NewNop())
				<-job.doneCh
				if job.err != nil {
					t.Fatal(job.err)
				}
			}
			record, err := coll.Find(job.owner, job.repo, job.address)
			if err != nil || record.HookID != "" || len(record.References) != 2 {
				t.Fatalf("unexpected manual webhook record: %+v err=%v", record, err)
			}
			for _, ref := range record.References {
				job.ref = ref
				removeWebhook(job, zap.NewNop())
				<-job.doneCh
				if job.err != nil {
					t.Fatal(job.err)
				}
			}
			if _, err := coll.Find(job.owner, job.repo, job.address); !errors.Is(err, mongo.ErrNoDocuments) {
				t.Fatalf("last reference was not removed: %v", err)
			}
		})
	}
	if remoteCalls.Load() != 0 {
		t.Fatalf("manual mode made %d remote requests", remoteCalls.Load())
	}
}
