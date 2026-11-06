package models

type WorkflowLogWindow struct {
	Target WorkflowTarget `json:"target"`
	Offset int            `json:"offset" jsonschema:"description=从日志末尾向前跳过的行数，零到五千"`
	Limit  int            `json:"limit" jsonschema:"description=一到五百行"`
}
