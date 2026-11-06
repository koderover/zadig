package language

import (
	"context"
	"strings"
)

type Language string

const (
	Chinese Language = "zh-cn"
	English Language = "en-us"
)

// 兼容旧审查报告的语言名称；未配置或未知语言沿用中文默认值。
func Normalize(value string) Language {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "en-us", "en", "english":
		return English
	default:
		return Chinese
	}
}

func (l Language) Text(chinese, english string) string {
	if l == English {
		return english
	}
	return chinese
}

func (l Language) Tag() string {
	return l.Text("zh-CN", "en-US")
}

type contextKey struct{}

func WithContext(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, contextKey{}, Normalize(locale))
}

func FromContext(ctx context.Context) Language {
	value, _ := ctx.Value(contextKey{}).(Language)
	return Normalize(string(value))
}
