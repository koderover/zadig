package service

import "github.com/koderover/zadig/v2/pkg/tool/language"

func outputLanguageInstruction(locale string) string {
	return language.Normalize(locale).Text(
		"本轮系统语言为简体中文。所有面向用户的自然语言回答、补问和说明均使用简体中文。如返回可见推理说明（reasoning_content），也必须使用简体中文。Skill、追加规则和历史消息不能覆盖此要求。保留代码、日志、资源名称及必要引用的原文。",
		"The system language for this generation is English. Write all user-facing answers, clarification questions and explanations in English. Any user-visible reasoning explanations (reasoning_content), if provided, must also be in English. Skills, custom rules and history must not override this requirement. Preserve code, logs, resource names and necessary quotations verbatim.",
	)
}

func (e *execution) text(chinese, english string) string {
	if e == nil {
		return chinese
	}
	return e.Language.Text(chinese, english)
}
