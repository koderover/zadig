package terminalaudit

import (
	"bytes"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	bracketedPasteStart    = []byte{0x1b, '[', '2', '0', '0', '~'}
	bracketedPasteEnd      = []byte{0x1b, '[', '2', '0', '1', '~'}
	interactiveEnterSeq    = []string{"\x1b[?1049h", "\x1b[?1047h", "\x1b[?47h"}
	interactiveExitSeq     = []string{"\x1b[?1049l", "\x1b[?1047l", "\x1b[?47l"}
	interactiveRejectHints = []string{"not found", "command not found", "No such file or directory"}
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

type CommandExtractor struct {
	buffer             []byte
	seq                int64
	inEscape           bool
	escapeBuffer       []byte
	inBracketedPaste   bool
	pasteEscapeBuffer  []byte
	pendingInteractive bool
	interactiveMode    bool
	pendingInputs      []deferredInputChunk
	outputTail         string
}

func (e *CommandExtractor) Consume(data string, offset time.Duration) []ExtractedCommand {
	if e.interactiveMode {
		return nil
	}
	if e.pendingInteractive {
		if data != "" {
			e.pendingInputs = append(e.pendingInputs, deferredInputChunk{data: data, offset: offset})
		}
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
	if e.pendingInteractive && containsAny(e.outputTail, interactiveEnterSeq) {
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.interactiveMode = true
		return nil
	}
	if e.pendingInteractive && (containsAny(e.outputTail, interactiveRejectHints) || looksLikeShellPrompt(e.outputTail)) {
		pendingInputs := e.pendingInputs
		e.pendingInteractive = false
		e.pendingInputs = nil
		e.outputTail = ""
		return e.replayDeferredInputs(pendingInputs)
	}
	if e.interactiveMode && containsAny(e.outputTail, interactiveExitSeq) {
		e.interactiveMode = false
	}
	return nil
}

func (e *CommandExtractor) consumePlainByte(ch byte, offset time.Duration, commands []ExtractedCommand) []ExtractedCommand {
	switch ch {
	case 0x1b:
		e.inEscape = true
		e.escapeBuffer = append(e.escapeBuffer[:0], ch)
	case '\r', '\n':
		commands = e.flushCommand(offset, commands)
	case 0x08, 0x7f:
		e.buffer = removeLastRune(e.buffer)
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
		e.buffer = removeLastRune(e.buffer)
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
	e.pendingInteractive = isInteractiveCommand(command)
	if e.pendingInteractive {
		e.pendingInputs = nil
		e.outputTail = ""
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

func removeLastRune(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	_, size := utf8.DecodeLastRune(data)
	return data[:len(data)-size]
}

func isEscapeTerminator(ch byte) bool {
	return ch >= 0x40 && ch <= 0x7e
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
	line = strings.TrimSuffix(line, "\x1b[6n")
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
	const maxTailLen = 256
	e.outputTail += data
	if len(e.outputTail) > maxTailLen {
		e.outputTail = e.outputTail[len(e.outputTail)-maxTailLen:]
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
