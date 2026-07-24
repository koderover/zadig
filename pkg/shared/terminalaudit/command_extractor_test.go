package terminalaudit

import (
	"testing"
	"time"
)

func TestCommandExtractorBackspaceRemovesCompleteUTF8Rune(t *testing.T) {
	extractor := NewCommandExtractor()

	commands := extractor.Consume("你\x7f好\r", time.Second)

	if len(commands) != 1 {
		t.Fatalf("command count = %d, want 1", len(commands))
	}
	if commands[0].Command != "好" {
		t.Fatalf("command = %q, want %q", commands[0].Command, "好")
	}
}

func TestCommandExtractorRecognizesInteractiveCommandPath(t *testing.T) {
	extractor := NewCommandExtractor()

	commands := extractor.Consume("/usr/bin/vim /tmp/file\r", time.Second)
	if len(commands) != 1 || commands[0].Command != "/usr/bin/vim /tmp/file" {
		t.Fatalf("initial commands = %#v", commands)
	}
	extractor.ObserveOutput("\x1b[?1049h")

	if commands := extractor.Consume(":q\r", 2*time.Second); len(commands) != 0 {
		t.Fatalf("interactive input was recorded as commands: %#v", commands)
	}
}
