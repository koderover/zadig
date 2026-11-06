package models

type Recommendation struct {
	SkillID string      `json:"skill_id"`
	Title   string      `json:"title"`
	Prompt  string      `json:"prompt"`
	Context PageContext `json:"context"`
}
