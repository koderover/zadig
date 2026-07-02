package jobcontroller

import (
	"testing"

	commonmodels "github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/repository/models"
)

func TestBuildAITestStatisticsFromReports(t *testing.T) {
	stats := buildAITestStatisticsFromReports([]*commonmodels.CustomWorkflowTestReport{
		{
			JobTaskName:    "test-0",
			TestName:       "suite-a",
			ZadigTestName:  "unit-test",
			TestCaseNum:    10,
			SuccessCaseNum: 9,
			SkipCaseNum:    1,
			FailedCaseNum:  1,
			TestTime:       1.23,
		},
		{
			JobTaskName:    "test-1",
			TestName:       "suite-b",
			ZadigTestName:  "unit-test",
			TestCaseNum:    5,
			SuccessCaseNum: 3,
			ErrorCaseNum:   2,
			TestTime:       2.34,
		},
	})

	if stats == nil {
		t.Fatalf("expected test statistics")
	}
	if got, want := stats.TestCaseNum, 15; got != want {
		t.Fatalf("unexpected total cases, got %d want %d", got, want)
	}
	if got, want := stats.SuccessCaseNum, 12; got != want {
		t.Fatalf("unexpected success cases, got %d want %d", got, want)
	}
	if got, want := stats.PassRate, 80.0; got != want {
		t.Fatalf("unexpected pass rate, got %v want %v", got, want)
	}
	if got, want := len(stats.Reports), 2; got != want {
		t.Fatalf("unexpected report count, got %d want %d", got, want)
	}
	if got, want := stats.Reports[0].PassRate, 90.0; got != want {
		t.Fatalf("unexpected report pass rate, got %v want %v", got, want)
	}
}

func TestParseAIReleaseSpecialistResultRejectsUnknownConclusion(t *testing.T) {
	_, err := ParseAIReleaseSpecialistResult(`{"conclusion":"reject","summary":"test","checks":[]}`)
	if err == nil {
		t.Fatalf("expected error for unknown conclusion")
	}
	if got, want := err.Error(), "invalid conclusion: reject"; got != want {
		t.Fatalf("unexpected error, got %q want %q", got, want)
	}
}

func TestParseAIReleaseSpecialistResultAcceptsNormalizedConclusion(t *testing.T) {
	result, err := ParseAIReleaseSpecialistResult(`{"conclusion":"passed","summary":"test","checks":[]}`)
	if err != nil {
		t.Fatalf("expected normalized conclusion to pass, got error: %v", err)
	}
	if got, want := result.Conclusion, "pass"; got != want {
		t.Fatalf("unexpected conclusion, got %q want %q", got, want)
	}
}
