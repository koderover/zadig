package common

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
)

func TestRequestValidationAndCursor(t *testing.T) {
	for _, req := range []models.RunRequest{
		{RequestID: "bad id", Message: "你好"},
		{RequestID: "valid", Message: " "},
		{RequestID: "valid", Message: strings.Repeat("a", 16001)},
		{RequestID: "valid", Message: "你好", Context: models.PageContext{TaskID: -1}},
	} {
		if ValidateRequest(req) == nil {
			t.Fatalf("应拒绝无效请求: %+v", req)
		}
	}

	if err := ValidateRequest(models.RunRequest{RequestID: "req-1", Message: "你好"}); err != nil {
		t.Fatal(err)
	}

	for _, value := range []string{"bad", strings.Repeat("a", 513)} {
		if _, err := DecodeCursor(value); !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("应拒绝无效游标: %v", err)
		}
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	c, err := DecodeCursor(EncodeCursor(now, "123"))
	if err != nil || c.ID != "123" || !c.At.Equal(now) {
		t.Fatalf("游标往返失败: %+v %v", c, err)
	}
}
