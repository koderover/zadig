package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestReadLogExcerptPagesFromTail(t *testing.T) {
	var input strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&input, "第%d行\n", i)
	}

	first, err := ReadLogExcerpt(context.Background(), strings.NewReader(input.String()), 0, 2)
	if err != nil || first.Text != "第999行\n第1000行" || first.NextOffset != 2 || first.TotalLines != 1000 {
		t.Fatalf("末尾窗口不正确: %+v %v", first, err)
	}

	previous, err := ReadLogExcerpt(context.Background(), strings.NewReader(input.String()), first.NextOffset, 2)
	if err != nil || previous.Text != "第997行\n第998行" {
		t.Fatalf("向前分页不正确: %+v %v", previous, err)
	}
}

func TestReadLogExcerptEnforcesLimitsAndCancellation(t *testing.T) {
	result, err := ReadLogExcerpt(context.Background(), strings.NewReader(strings.Repeat(strings.Repeat("字", 500)+"\n", 500)), 0, 500)
	if err != nil || len(result.Text) > 48<<10 || !result.Truncated {
		t.Fatalf("输出必须限长: %+v %v", result, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadLogExcerpt(ctx, strings.NewReader("日志\n"), 0, 1); err == nil {
		t.Fatal("取消后不能继续扫描")
	}

	if _, err := ReadLogExcerpt(context.Background(), strings.NewReader(""), 5001, 1); err == nil {
		t.Fatal("必须限制扫描窗口")
	}
}
