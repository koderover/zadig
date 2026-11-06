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

type SkillUpdate struct {
	Enabled     *bool   `json:"enabled" binding:"required"`
	CustomRules *string `json:"custom_rules" binding:"required"`
}
