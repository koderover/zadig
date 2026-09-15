package handler

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"

	e "github.com/koderover/zadig/v2/pkg/tool/errors"
)

func TestOpenAPIProjectLookupError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
		desc string
	}{
		{
			name: "project does not exist",
			err:  mongo.ErrNoDocuments,
			code: e.ErrNotFound.Code(),
			desc: "产品不存在: missing-project",
		},
		{
			name: "repository error",
			err:  errors.New("database unavailable"),
			code: e.ErrGetProduct.Code(),
			desc: "database unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := openAPIProjectLookupError("missing-project", tt.err)
			httpErr, ok := err.(*e.HTTPError)
			if !ok {
				t.Fatalf("expected HTTPError, got %T", err)
			}
			if httpErr.Code() != tt.code {
				t.Fatalf("code = %d, want %d", httpErr.Code(), tt.code)
			}
			if httpErr.Desc() != tt.desc {
				t.Fatalf("description = %q, want %q", httpErr.Desc(), tt.desc)
			}
		})
	}
}
