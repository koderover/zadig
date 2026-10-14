package step

import (
	"encoding/json"
	"testing"
)

func TestAIReviewReportValidation(t *testing.T) {
	for _, test := range []struct {
		raw   string
		valid bool
	}{
		{`null`, false},
		{`{}`, false},
		{`{"duration_ms":0,"incomplete":false,"exit_code":0}`, false},
		{`{"duration_ms":-1,"incomplete":false,"exit_code":0}`, false},
		{`{"duration_ms":1}`, false},
		{`{"duration_ms":1,"incomplete":false,"exit_code":0}`, true},
		{`{"duration_ms":1,"incomplete":true,"exit_code":1}`, true},
		{`{"duration_ms":1,"incomplete":true,"exit_code":2,"errors":["timeout"]}`, true},
		{`{"duration_ms":1,"incomplete":false,"exit_code":-1}`, true},
		{`{"duration_ms":1,"incomplete":false,"exit_code":137}`, true},
		{`{"duration_ms":1,"incomplete":false,"exit_code":"0"}`, false},
		{`{"duration_ms":1,"incomplete":false}`, false},
		{`{"duration_ms":1,"exit_code":0}`, false},
		{`{"duration_ms":1,"incomplete":null,"exit_code":0}`, false},
		{`{"duration_ms":1,"incomplete":false,"exit_code":null}`, false},
		{`{"duration_ms":1,"incomplete":false,"exit_code":1.5}`, false},
		{`{"duration_ms":1,"incomplete":"false","exit_code":0}`, false},
	} {
		t.Run(test.raw, func(t *testing.T) {
			var report *AIReviewReport
			err := json.Unmarshal([]byte(test.raw), &report)
			if err == nil {
				err = report.Validate()
			}
			if (err == nil) != test.valid {
				t.Fatalf("valid=%v, want %v: %v", err == nil, test.valid, err)
			}
		})
	}
}
