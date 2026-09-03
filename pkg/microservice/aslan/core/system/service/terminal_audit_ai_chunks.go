package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/koderover/zadig/v2/pkg/microservice/aslan/core/common/service/terminalaudit"
)

// buildTerminalAuditAIChunks packs complete records when possible and splits
// oversized records into bounded chunks that share one serial group.
func buildTerminalAuditAIChunks(evidence *terminalaudit.TerminalAuditEvidence) (chunks []terminalAuditAIChunk, coveredCommands int, truncated bool) {
	chunks = make([]terminalAuditAIChunk, 0, maxTerminalAuditAIChunks)
	var chunk strings.Builder
	chunkCommands := make(map[int64]string)
	chunkRunes := 0
	nextSerialGroup := 0

	flushChunk := func() {
		chunks = append(chunks, terminalAuditAIChunk{
			evidence:    chunk.String(),
			commands:    chunkCommands,
			serialGroup: nextSerialGroup,
		})
		nextSerialGroup++
		chunk.Reset()
		chunkCommands = make(map[int64]string)
		chunkRunes = 0
	}

	appendRecord := func(label, data string, command *terminalaudit.TerminalAuditCommandEvidence) bool {
		if chunk.Len() == 0 && len(chunks) >= maxTerminalAuditAIChunks {
			truncated = true
			return false
		}

		record := fmt.Sprintf("[%s]\n%s", label, data)
		recordRunes := utf8.RuneCountInString(record)
		if recordRunes > maxTerminalAuditAIChunkRunes {
			if chunk.Len() > 0 {
				flushChunk()
			}
			parts := splitTerminalAuditAIRecord(label, data)
			remainingChunks := maxTerminalAuditAIChunks - len(chunks)
			if len(parts) > remainingChunks {
				parts = parts[:remainingChunks]
				truncated = true
			}
			serialGroup := nextSerialGroup
			nextSerialGroup++
			for _, part := range parts {
				commands := map[int64]string{command.Seq: command.Command}
				chunks = append(chunks, terminalAuditAIChunk{evidence: part, commands: commands, serialGroup: serialGroup})
			}
			return true
		}
		separatorRunes := 0
		if chunk.Len() > 0 {
			separatorRunes = 2
		}
		if chunk.Len() > 0 && chunkRunes+separatorRunes+recordRunes > maxTerminalAuditAIChunkRunes {
			flushChunk()
			if len(chunks) >= maxTerminalAuditAIChunks {
				truncated = true
				return false
			}
		}
		if chunk.Len() > 0 {
			chunk.WriteString("\n\n")
			chunkRunes += 2
		}
		chunk.WriteString(record)
		chunkRunes += recordRunes
		chunkCommands[command.Seq] = command.Command
		return true
	}

	// Commands are already sorted by session order when the evidence is built.
	for i := range evidence.Commands {
		command := &evidence.Commands[i]
		commandData, _ := json.Marshal(command)
		if !appendRecord(fmt.Sprintf("command seq=%d", command.Seq), string(commandData), command) {
			break
		}
		coveredCommands++
	}
	if chunk.Len() > 0 {
		flushChunk()
	}
	return chunks, coveredCommands, truncated
}

func splitTerminalAuditAIRecord(label, data string) []string {
	dataRunes := []rune(data)
	parts := make([]string, 0, len(dataRunes)/maxTerminalAuditAIChunkRunes+1)
	for part := 1; len(dataRunes) > 0; part++ {
		prefix := fmt.Sprintf("[%s continuation=%d]\n", label, part)
		payloadRunes := maxTerminalAuditAIChunkRunes - utf8.RuneCountInString(prefix)
		if payloadRunes > len(dataRunes) {
			payloadRunes = len(dataRunes)
		}
		parts = append(parts, prefix+string(dataRunes[:payloadRunes]))
		dataRunes = dataRunes[payloadRunes:]
	}
	return parts
}
