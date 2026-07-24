package terminalaudit

import "testing"

func TestRecorderIgnoresEventsAfterCloseStarts(t *testing.T) {
	recorder := &asciicastRecorder{
		closed:    true,
		sanitizer: noopSanitizer{},
	}

	recorder.RecordInput("input")
	recorder.RecordOutput("output")
	recorder.RecordResize(80, 24)
}
