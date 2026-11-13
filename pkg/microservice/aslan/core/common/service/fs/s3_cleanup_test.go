package fs

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"go.uber.org/zap"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
	s3service "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/s3"
)

func TestDeleteArchivedFilesFromS3(t *testing.T) {
	for _, tc := range []struct {
		name     string
		names    []string
		wantKeys []string
		status   int
	}{
		{name: "only the requested revision", names: []string{"service-123"}, wantKeys: []string{"prefix/project/service/service-123.tar.gz"}, status: http.StatusOK},
		{name: "multiple archives", names: []string{"service", "service-123"}, wantKeys: []string{"prefix/project/service/service.tar.gz", "prefix/project/service/service-123.tar.gz"}, status: http.StatusOK},
		{name: "storage error is returned", names: []string{"service-123"}, wantKeys: []string{"prefix/project/service/service-123.tar.gz"}, status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var keys []string
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || !r.URL.Query().Has("delete") {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var request struct {
					Objects []struct {
						Key string `xml:"Key"`
					} `xml:"Object"`
				}
				if err := xml.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				mu.Lock()
				for _, object := range request.Objects {
					keys = append(keys, object.Key)
				}
				mu.Unlock()
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(tc.status)
				if tc.status != http.StatusOK {
					_, _ = w.Write([]byte("<Error><Code>InvalidRequest</Code><Message>test failure</Message></Error>"))
					return
				}
				_, _ = w.Write([]byte("<DeleteResult xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\"/>"))
			}))
			defer server.Close()
			storage := &s3service.S3{S3Storage: &models.S3Storage{
				Endpoint: server.URL, Ak: "test-key", Sk: "test-secret", Bucket: "bucket", Subfolder: "prefix", Insecure: true,
			}}
			err := deleteArchivedFilesFromS3(tc.names, "project/service", storage, zap.NewNop().Sugar())
			if (err != nil) != (tc.status != http.StatusOK) {
				t.Fatalf("unexpected delete error: %v", err)
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(keys, tc.wantKeys) {
				t.Fatalf("deleted keys %v, want %v", keys, tc.wantKeys)
			}
		})
	}
}

func TestDeleteArchivedFileFromS3EmptyNames(t *testing.T) {
	if err := DeleteArchivedFileFromS3(nil, "unused", zap.NewNop().Sugar()); err != nil {
		t.Fatal(err)
	}
}
