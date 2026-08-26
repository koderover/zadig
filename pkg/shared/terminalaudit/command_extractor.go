package terminalaudit

import (
	"bytes"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	terminalEscapeByte byte = '\x1b'
	terminalDeleteByte byte = '\x7f'

	// A bracket-led terminal control sequence ends with a byte in this protocol-defined range.
	controlSequenceFinalByteMin byte = 0x40
	controlSequenceFinalByteMax byte = 0x7e
	escapeSequenceTypeIndex          = 1 // Byte immediately following ESC.
	escapeSequencePrefixLength       = 2 // ESC followed by the sequence type, such as '[' or ']'.
	cursorPositionQuery              = "\x1b[6n"

	// Bound memory retained while an interactive command is starting or a command is being entered.
	maxDeferredInputBytes = 64 * 1024
	maxCommandBytes       = 64 * 1024
	// Keep enough recent output to recognize sequences or shell prompts split across WebSocket messages.
	maxInteractiveOutputTailBytes = 256
)

var (
	// Terminals wrap pasted text with these markers so it can be distinguished from typed input.
	bracketedPasteStart = []byte("\x1b[200~")
	bracketedPasteEnd   = []byte("\x1b[201~")
	// Title, clipboard, and device-control payloads end with ESC followed by a backslash.
	terminalStringEnd = []byte{terminalEscapeByte, '\\'}

	// Full-screen programs such as vim and top use these sequences to enter and leave the alternate screen.
	alternateScreenEnterSequences = []string{"\x1b[?1049h", "\x1b[?1047h", "\x1b[?47h"}
	alternateScreenExitSequences  = []string{"\x1b[?1049l", "\x1b[?1047l", "\x1b[?47l"}

	// These messages indicate that a full-screen command failed and normal shell input should resume.
	interactiveCommandFailureHints = []string{"not found", "No such file or directory"}
)

type ExtractedCommand struct {
	Seq          int64
	Command      string
	TimeOffsetMS int64
}

type deferredInputChunk struct {
	data   string
	offset time.Duration
}

// CommandExtractor reconstructs shell commands from raw PTY input. It removes terminal
// control sequences and pauses command extraction while a full-screen program is active.
type CommandExtractor struct {
	buffer                 []byte
	seq                    int64
	inEscape               bool
	escapeBuffer           []byte
	inBracketedPaste       bool
	pasteEscapeBuffer      []byte
	pendingInteractive     bool
	interactiveMode        bool
	pendingInputs          []deferredInputChunk
	pendingInputBytes      int
	discardingPendingInput bool
	discardingCommand      bool
	outputTail             string
}

func (e *CommandExtractor) Consume(data string, offset time.Duration) []ExtractedCommand {
	// PTY input can split one control sequence across multiple WebSocket messages,
	// so parsing state is retained between Consume calls.
	if e.interactiveMode {
		return nil
	}
	if e.pendingInteractive {
		if data == "" || e.discardingPendingInput {
			return nil
		}
		if len(data) > maxDeferredInputBytes-e.pendingInputBytes {
			// Keep the already-buffered prefix for replay; only drop the overflow tail.
			e.discardingPendingInput = true
			return nil
		}
		e.pendingInputs = append(e.pendingInputs, deferredInputChunk{data: data, offset: offset})
		e.pendingInputBytes += len(data)
		return nil
	}
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

func (e *CommandExtractor) ObserveOutput(data string) []ExtractedCommand {
	if data == "" {
		return nil
	}
	e.appendOutputTail(data)
	if e.pendingInteractive && containsAny(e.outputTail, alternateScreenEnterSequences) {
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.pendingInputBytes = 0
		e.discardingPendingInput = false
		e.interactiveMode = true
		return nil
	}
	if e.pendingInteractive && containsAny(e.outputTail, interactiveCommandFailureHints) {
		pendingInputs := e.pendingInputs
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.pendingInputBytes = 0
		e.discardingPendingInput = false
		e.outputTail = ""
		return e.replayDeferredInputs(pendingInputs)
	}
	if e.pendingInteractive && looksLikeShellPrompt(e.outputTail) {
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.pendingInputBytes = 0
		e.discardingPendingInput = false
		e.outputTail = ""
		return nil
	}
	if e.interactiveMode && containsAny(e.outputTail, alternateScreenExitSequences) {
		e.interactiveMode = false
	}
	return nil
}

func (e *CommandExtractor) Flush() []ExtractedCommand {
	commands := make([]ExtractedCommand, 0)
	// Replaying deferred input can discover another interactive command and queue
	// more input, so drain until no pending input remains.
	for len(e.pendingInputs) > 0 {
		pendingInputs := e.pendingInputs
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.pendingInputBytes = 0
		e.discardingPendingInput = false
		e.outputTail = ""
		commands = append(commands, e.replayDeferredInputs(pendingInputs)...)
	}
	return commands
}

// consumePlainByte parses terminal input; ESC starts a terminal control sequence.
// consumePastedByte intentionally keeps ESC as command content while bracketed paste is active.
func (e *CommandExtractor) consumePlainByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	switch ch {
	case terminalEscapeByte:
		e.inEscape = true
		e.escapeBuffer = append(e.escapeBuffer[:0], ch)
	case '\r', '\n':
		commands = e.flushCommand(offset, commands)
	case '\b', terminalDeleteByte:
		e.buffer = removeLastRune(e.buffer)
	default:
		if ch >= ' ' || ch == '\t' {
			e.appendCommandByte(ch)
		}
	}
	return commands
}

func (e *CommandExtractor) consumeEscapeByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	e.escapeBuffer = append(e.escapeBuffer, ch)
	if len(e.escapeBuffer) < escapeSequencePrefixLength {
		return commands
	}

	switch e.escapeBuffer[escapeSequenceTypeIndex] {
	case '[': // Bracket-led terminal control sequence, terminated by a protocol-defined final byte.
		if len(e.escapeBuffer) == escapeSequencePrefixLength {
			return commands
		}
		if !isControlSequenceFinalByte(ch) {
			if len(e.escapeBuffer) > len(bracketedPasteStart) {
				e.escapeBuffer = e.escapeBuffer[:len(bracketedPasteStart)]
			}
			return commands
		}
		if bytes.Equal(e.escapeBuffer, bracketedPasteStart) {
			e.inBracketedPaste = true
			e.pasteEscapeBuffer = e.pasteEscapeBuffer[:0]
		}
		e.resetEscape()
		return commands
	case ']', 'P': // Terminal metadata or device-control payload, not command content.
		if e.escapeEndsWithStringTerminator() || e.escapeBuffer[escapeSequenceTypeIndex] == ']' && ch == '\a' {
			e.resetEscape()
			return commands
		}
		if len(e.escapeBuffer) > escapeSequencePrefixLength+1 {
			e.escapeBuffer = append(e.escapeBuffer[:escapeSequencePrefixLength], e.escapeBuffer[len(e.escapeBuffer)-1])
		}
		return commands
	case 'O': // Function and keypad key sequence, which contains one payload byte.
		if len(e.escapeBuffer) < escapeSequencePrefixLength+1 {
			return commands
		}
		e.resetEscape()
		return commands
	default:
		// Not a recognized escape introducer: drop the lone ESC and reprocess
		// the current byte as plain text rather than swallowing it.
		e.resetEscape()
		return e.consumePlainByte(ch, offset, commands)
	}
}

func (e *CommandExtractor) consumeBracketedPasteByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	if len(e.pasteEscapeBuffer) > 0 {
		return e.consumePasteEscapeByte(ch, offset, commands)
	}
	if ch == terminalEscapeByte {
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
	case terminalEscapeByte:
		e.appendCommandByte(ch)
	case '\r', '\n':
		commands = e.flushCommand(offset, commands)
	case '\b', terminalDeleteByte:
		e.buffer = removeLastRune(e.buffer)
	default:
		if ch >= ' ' || ch == '\t' {
			e.appendCommandByte(ch)
		}
	}
	return commands
}

func (e *CommandExtractor) flushCommand(offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	if e.discardingCommand {
		e.buffer = nil
		e.discardingCommand = false
		return commands
	}
	command := strings.TrimSpace(string(e.buffer))
	e.buffer = nil
	if command == "" {
		return commands
	}
	e.pendingInteractive = isInteractiveCommand(command)
	if e.pendingInteractive {
		e.pendingInputs = nil
		e.pendingInputBytes = 0
		e.discardingPendingInput = false
		e.outputTail = ""
	}
	e.seq++
	return append(commands, ExtractedCommand{
		Seq:          e.seq,
		Command:      command,
		TimeOffsetMS: offset.Milliseconds(),
	})
}

func (e *CommandExtractor) appendCommandByte(ch byte) {
	if e.discardingCommand {
		return
	}
	if len(e.buffer) >= maxCommandBytes {
		e.buffer = nil
		e.discardingCommand = true
		return
	}
	e.buffer = append(e.buffer, ch)
}

func (e *CommandExtractor) resetEscape() {
	e.inEscape = false
	e.escapeBuffer = nil
}

func (e *CommandExtractor) escapeEndsWithStringTerminator() bool {
	return bytes.HasSuffix(e.escapeBuffer, terminalStringEnd)
}

func removeLastRune(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	_, size := utf8.DecodeLastRune(data)
	return data[:len(data)-size]
}

func isControlSequenceFinalByte(ch byte) bool {
	return ch >= controlSequenceFinalByteMin && ch <= controlSequenceFinalByteMax
}

func containsAny(data string, targets []string) bool {
	for _, target := range targets {
		if strings.Contains(data, target) {
			return true
		}
	}
	return false
}

func looksLikeShellPrompt(data string) bool {
	line := data
	if idx := strings.LastIndex(line, "\n"); idx >= 0 {
		line = line[idx+1:]
	}
	line = strings.TrimSuffix(line, cursorPositionQuery)
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	return strings.HasSuffix(line, "$") ||
		strings.HasSuffix(line, "#") ||
		strings.HasSuffix(line, ">") ||
		strings.HasSuffix(line, "%")
}

func (e *CommandExtractor) appendOutputTail(data string) {
	e.outputTail += data
	if len(e.outputTail) > maxInteractiveOutputTailBytes {
		e.outputTail = strings.Clone(e.outputTail[len(e.outputTail)-maxInteractiveOutputTailBytes:])
	}
}

func (e *CommandExtractor) replayDeferredInputs(chunks []deferredInputChunk) []ExtractedCommand {
	commands := make([]ExtractedCommand, 0)
	for _, chunk := range chunks {
		commands = append(commands, e.Consume(chunk.data, chunk.offset)...)
	}
	return commands
}

func isInteractiveCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return false
	}
	// 这里只覆盖已知会切换全屏/交互界面的常见命令，用于避免命令列表被编辑器或 TUI 内部输入污染。
	// 不在名单内的交互程序仍按输入流提取命令，后续如果需要再按真实场景补充。
	switch path.Base(fields[0]) {
	case "vi", "vim", "nvim", "view", "vimdiff",
		"nano", "pico", "emacs",
		"less", "more", "most", "pg", "man",
		"top", "htop", "btop", "atop", "iftop", "iotop", "glances", "nload", "nvtop", "watch",
		"tig", "lazygit", "k9s", "ranger", "mc", "nnn":
		return true
	default:
		return false
	}
}
