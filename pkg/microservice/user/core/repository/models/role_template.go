package models

type RoleTemplate struct {
	ID          uint   `gorm:"primarykey"         json:"id"`
	Name        string `gorm:"column:name"        json:"name"`
	Description string `gorm:"column:description" json:"description"`
	Type        int    `gorm:"column:type" json:"type"`

	RoleTemplateActionBindings []RoleTemplateActionBinding `gorm:"foreignKey:RoleTemplateID;constraint:OnDelete:CASCADE;" json:"-"`
	RoleTemplateBindings       []RoleTemplateBinding       `gorm:"foreignKey:RoleTemplateID;constraint:OnDelete:CASCADE;" json:"-"`
}

func (RoleTemplate) TableName() string {
	return "role_template"
}
