package terminalaudit

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRecorderIgnoresEventsAfterCloseStarts(t *testing.T) {
	recorder := &asciicastRecorder{
		closed:     true,
		inputMask:  newStreamSanitizer(nil, nil),
		outputMask: newStreamSanitizer(nil, nil),
	}

	recorder.RecordInput("input")
	recorder.RecordOutput("output")
	recorder.RecordResize(80, 24)
}

func TestRecorderWritesEachOutputOnce(t *testing.T) {
	var cast bytes.Buffer
	live := newLivePublisher("session-output", newFakeLiveTransport())
	recorder := &asciicastRecorder{
		startedAt:  time.Now(),
		inputMask:  newStreamSanitizer(nil, nil),
		outputMask: newStreamSanitizer(nil, nil),
		extractor:  NewCommandExtractor(),
		writer:     bufio.NewWriter(&cast),
		live:       live,
	}

	recorder.RecordOutput("hello")
	if err := recorder.writer.Flush(); err != nil {
		t.Fatalf("flush cast: %v", err)
	}
	live.close()

	if lines := strings.Count(cast.String(), "\n"); lines != 1 {
		t.Fatalf("output event count = %d, want 1", lines)
	}
}

func TestRecorderTerminatesSessionOnRecordFailure(t *testing.T) {
	terminated := make(chan struct{})
	recorder := &asciicastRecorder{
		terminate: func() { close(terminated) },
	}

	recorder.fail(errors.New("storage unavailable"))
	recorder.fail(errors.New("another failure"))

	select {
	case <-terminated:
	case <-time.After(time.Second):
		t.Fatal("record failure did not terminate session")
	}
}

func TestRecorderCloseReturnsFirstCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	recorder := &asciicastRecorder{closeErr: closeErr}
	recorder.closeOnce.Do(func() {})

	if err := recorder.Close(""); !errors.Is(err, closeErr) {
		t.Fatalf("close error = %v, want %v", err, closeErr)
	}
}
