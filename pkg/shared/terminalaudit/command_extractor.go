package terminalaudit

import (
	"strings"
	"time"
)

type ExtractedCommand struct {
	Seq          int64
	Command      string
	TimeOffsetMS int64
}

type CommandExtractor struct {
	buffer           []byte
	seq              int64
	inEscape         bool
	escapeBodyBegins bool
}

func NewCommandExtractor() *CommandExtractor {
	return &CommandExtractor{}
}

func (e *CommandExtractor) Consume(data string, offset time.Duration) []ExtractedCommand {
	commands := make([]ExtractedCommand, 0)
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if e.inEscape {
			if !e.escapeBodyBegins && (ch == '[' || ch == ']' || ch == 'O' || ch == 'P') {
				e.escapeBodyBegins = true
				continue
			}
			if isEscapeTerminator(ch) {
				e.inEscape = false
				e.escapeBodyBegins = false
			}
			continue
		}

		switch ch {
		case 0x1b:
			e.inEscape = true
			e.escapeBodyBegins = false
		case '\r', '\n':
			command := strings.TrimSpace(string(e.buffer))
			e.buffer = e.buffer[:0]
			if command == "" {
				continue
			}
			e.seq++
			commands = append(commands, ExtractedCommand{
				Seq:          e.seq,
				Command:      command,
				TimeOffsetMS: offset.Milliseconds(),
			})
		case 0x08, 0x7f:
			if len(e.buffer) > 0 {
				e.buffer = e.buffer[:len(e.buffer)-1]
			}
		default:
			if ch >= 0x20 || ch == '\t' {
				e.buffer = append(e.buffer, ch)
			}
		}
	}
	return commands
}

func isEscapeTerminator(ch byte) bool {
	return ch >= 0x40 && ch <= 0x7e
}
