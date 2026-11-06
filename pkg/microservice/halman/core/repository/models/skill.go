package models

import "time"

type Skill struct {
	ID          string    `json:"id"`
	Source      string    `json:"source"`
	Name        string    `json:"name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Content     string    `json:"content"`
	Enabled     bool      `json:"enabled"`
	CustomRules string    `json:"custom_rules"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (Skill) TableName() string { return "ai_skill" }

// SkillView 仅用于管理接口展示，不参与数据库存储或运行时指令组装。
type SkillView struct {
	Skill
	CategoryName string `json:"category_name"` // 随系统语言展示的分类名称，category 保留原始分类值。
}

type SkillUpdate struct {
	Enabled     *bool   `json:"enabled" binding:"required"`
	CustomRules *string `json:"custom_rules" binding:"required"`
}
