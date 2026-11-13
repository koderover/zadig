package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
)

type LogExcerpt struct {
	Text       string `json:"text"`
	Offset     int    `json:"offset"`
	NextOffset int    `json:"next_offset"` // 工作流向前分页游标；容器日志不支持分页，固定为 0。
	TotalLines int    `json:"total_lines"` // 容器日志的完整总行数未知，返回 -1。
	Truncated  bool   `json:"truncated"`
}

func ReadLogExcerpt(ctx context.Context, reader io.Reader, offset, limit int) (*LogExcerpt, error) {
	if offset < 0 || offset > 5000 || limit < 1 || limit > 500 {
		return nil, fmt.Errorf("日志窗口超出允许范围")
	}

	ring := make([]string, offset+limit)
	count := 0
	clipped := false
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 64<<10)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		line := scanner.Text()
		if len(line) > 1024 {
			line = strings.ToValidUTF8(line[:1024], "�") + "[行已截断]"
			clipped = true
		}

		ring[count%len(ring)] = line
		count++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("日志包含超长行或无法读取，请缩小日志来源")
	}

	end := count - offset
	if end < 0 {
		end = 0
	}

	start := end - limit
	if start < 0 {
		start = 0
	}

	var lines []string
	size := 0
	for i := end - 1; i >= start; i-- {
		line := ring[i%len(ring)]
		if size+len(line)+1 > 48<<10 {
			clipped = true
			break
		}

		lines = append(lines, line)
		size += len(line) + 1
	}

	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return &LogExcerpt{
		Text:       strings.Join(lines, "\n"),
		Offset:     offset,
		NextOffset: offset + len(lines),
		TotalLines: count,
		Truncated:  clipped || start > 0 || offset > 0,
	}, nil
}

func GetWorkflowLogExcerpt(ctx context.Context, workflow, job string, taskID int64, offset, limit int, logger *zap.SugaredLogger) (*LogExcerpt, error) {
	// shortcut: S3 仍下载整个步骤日志文件，下载量成为瓶颈时改用范围读取。
	file, cleanup, err := downloadContainerLog(workflow, job, taskID, logger)
	if err != nil {
		return nil, err
	}

	defer cleanup()
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	return ReadLogExcerpt(ctx, f, offset, limit)
}
