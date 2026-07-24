package terminalaudit

import "testing"

func TestStreamSanitizerMasksSecretAcrossChunks(t *testing.T) {
	sanitizer := newStreamSanitizer([]string{"secret"}, nil)

	if got := sanitizer.Write("sec"); got != "" {
		t.Fatalf("first chunk = %q, want buffered", got)
	}
	if got := sanitizer.Write("ret!"); got != "********!" {
		t.Fatalf("second chunk = %q, want masked output", got)
	}
}

func TestStreamSanitizerFlushesIncompletePrefix(t *testing.T) {
	sanitizer := newStreamSanitizer([]string{"secret"}, nil)

	if got := sanitizer.Write("sec"); got != "" {
		t.Fatalf("chunk = %q, want buffered", got)
	}
	if got := sanitizer.Flush(); got != "sec" {
		t.Fatalf("flushed chunk = %q, want original incomplete prefix", got)
	}
}

func TestStreamSanitizerMasksSecretEnvironmentValue(t *testing.T) {
	sanitizer := newStreamSanitizer(nil, []string{"TOKEN=secret=value"})

	if got := sanitizer.Write("secret=value"); got != "********" {
		t.Fatalf("masked environment value = %q", got)
	}
}

func TestStreamSanitizerPrefersLongestSecret(t *testing.T) {
	sanitizer := newStreamSanitizer([]string{"sec", "secret"}, nil)

	if got := sanitizer.Write("sec"); got != "" {
		t.Fatalf("short secret = %q, want buffered", got)
	}
	if got := sanitizer.Write("ret"); got != secretMask {
		t.Fatalf("long secret = %q, want one mask", got)
	}
}
