package terminalio

import "testing"

type outputRecorder struct {
	output string
}

func (r *outputRecorder) RecordInput(string)          {}
func (r *outputRecorder) RecordResize(uint16, uint16) {}
func (r *outputRecorder) RecordOutput(output string)  { r.output = output }

func TestProcessOutputRecordsWithoutChangingTerminalOutput(t *testing.T) {
	recorder := &outputRecorder{}

	output := ProcessOutput("secret output", recorder)

	if output != "secret output" {
		t.Fatalf("terminal output = %q, want original output", output)
	}
	if recorder.output != "secret output" {
		t.Fatalf("recorded output = %q, want original output", recorder.output)
	}
}
