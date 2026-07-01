package jobcontroller

import "testing"

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
