package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestWorkflowTaskListSettingsValidation(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"refresh_interval":3}`, true},
		{`{"refresh_interval":10}`, true},
		{`{}`, false},
		{`{"refresh_interval":0}`, false},
		{`{"refresh_interval":-1}`, false},
		{`{"refresh_interval":1.5}`, false},
		{`{"refresh_interval":"3"}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			req := httptest.NewRequest("PUT", "/", strings.NewReader(tc.body))
			err := binding.JSON.Bind(req, new(WorkflowTaskListSettings))
			if (err == nil) != tc.valid {
				t.Fatalf("validation error = %v; want valid = %v", err, tc.valid)
			}
		})
	}
}
