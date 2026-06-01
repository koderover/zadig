package terminalaudit

import (
	"bytes"
	"strings"
	"time"
)

var (
	bracketedPasteStart = []byte{0x1b, '[', '2', '0', '0', '~'}
	bracketedPasteEnd   = []byte{0x1b, '[', '2', '0', '1', '~'}
)

type ExtractedCommand struct {
	Seq          int64
	Command      string
	TimeOffsetMS int64
}

type CommandExtractor struct {
	buffer            []byte
	seq               int64
	inEscape          bool
	escapeBuffer      []byte
	inBracketedPaste  bool
	pasteEscapeBuffer []byte
}

func NewCommandExtractor() *CommandExtractor {
	return &CommandExtractor{}
}

func (e *CommandExtractor) Consume(data string, offset time.Duration) []ExtractedCommand {
	commands := make([]ExtractedCommand, 0)
	for i := 0; i < len(data); i++ {
		ch := data[i]
		if e.inBracketedPaste {
			commands = e.consumeBracketedPasteByte(ch, offset, commands)
			continue
		}

		if e.inEscape {
			commands = e.consumeEscapeByte(ch, offset, commands)
			continue
		}

		commands = e.consumePlainByte(ch, offset, commands)
	}
	return commands
}

func (e *CommandExtractor) consumePlainByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	switch ch {
	case 0x1b:
		e.inEscape = true
		e.escapeBuffer = append(e.escapeBuffer[:0], ch)
	case '\r', '\n':
		commands = e.flushCommand(offset, commands)
	case 0x08, 0x7f:
		if len(e.buffer) > 0 {
			e.buffer = e.buffer[:len(e.buffer)-1]
		}
	default:
		if ch >= 0x20 || ch == '\t' {
			e.buffer = append(e.buffer, ch)
		}
	}
	return commands
}

func (e *CommandExtractor) consumeEscapeByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	e.escapeBuffer = append(e.escapeBuffer, ch)
	if len(e.escapeBuffer) < 2 {
		return commands
	}

	second := e.escapeBuffer[1]
	if second != '[' && second != ']' && second != 'O' && second != 'P' {
		e.resetEscape()
		return commands
	}
	if len(e.escapeBuffer) == 2 {
		return commands
	}

	if !isEscapeTerminator(ch) {
		return commands
	}

	if bytes.Equal(e.escapeBuffer, bracketedPasteStart) {
		e.inBracketedPaste = true
		e.pasteEscapeBuffer = e.pasteEscapeBuffer[:0]
	}
	e.resetEscape()
	return commands
}

func (e *CommandExtractor) consumeBracketedPasteByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	if len(e.pasteEscapeBuffer) > 0 {
		return e.consumePasteEscapeByte(ch, offset, commands)
	}
	if ch == 0x1b {
		e.pasteEscapeBuffer = append(e.pasteEscapeBuffer[:0], ch)
		return commands
	}
	return e.consumePastedByte(ch, offset, commands)
}

func (e *CommandExtractor) consumePasteEscapeByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	e.pasteEscapeBuffer = append(e.pasteEscapeBuffer, ch)
	if bytes.Equal(e.pasteEscapeBuffer, bracketedPasteEnd) {
		e.inBracketedPaste = false
		e.pasteEscapeBuffer = e.pasteEscapeBuffer[:0]
		return commands
	}
	if bytes.HasPrefix(bracketedPasteEnd, e.pasteEscapeBuffer) {
		return commands
	}
	for _, pasteCh := range e.pasteEscapeBuffer {
		commands = e.consumePastedByte(pasteCh, offset, commands)
	}
	e.pasteEscapeBuffer = e.pasteEscapeBuffer[:0]
	return commands
}

func (e *CommandExtractor) consumePastedByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	switch ch {
	case 0x1b:
		e.buffer = append(e.buffer, ch)
	case '\r', '\n':
		commands = e.flushCommand(offset, commands)
	case 0x08, 0x7f:
		if len(e.buffer) > 0 {
			e.buffer = e.buffer[:len(e.buffer)-1]
		}
	default:
		if ch >= 0x20 || ch == '\t' {
			e.buffer = append(e.buffer, ch)
		}
	}
	return commands
}

func (e *CommandExtractor) flushCommand(offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	command := strings.TrimSpace(string(e.buffer))
	e.buffer = e.buffer[:0]
	if command == "" {
		return commands
	}
	e.seq++
	return append(commands, ExtractedCommand{
		Seq:          e.seq,
		Command:      command,
		TimeOffsetMS: offset.Milliseconds(),
	})
}

func (e *CommandExtractor) resetEscape() {
	e.inEscape = false
	e.escapeBuffer = e.escapeBuffer[:0]
}

func isEscapeTerminator(ch byte) bool {
	return ch >= 0x40 && ch <= 0x7e
}
