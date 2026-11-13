package orm

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/common"
	"github.com/koderover/zadig/v2/pkg/microservice/halman/core/repository/models"
	builtins "github.com/koderover/zadig/v2/pkg/microservice/halman/core/skill"
)

func (s *Store) syncBuiltinSkills(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, item := range builtins.Builtins() {
			var existing models.Skill
			err := tx.Where("id=?", item.ID).Take(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&item).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if existing.Source != "builtin" {
				return fmt.Errorf("内置 Skill 标识与其他来源冲突: %s", item.ID)
			}
			if existing.Name == item.Name && existing.Category == item.Category && existing.Description == item.Description && existing.Content == item.Content {
				continue
			}
			// 升级仅更新内置定义，不覆盖管理员配置和其他来源的 Skill。
			if err := tx.Model(&models.Skill{}).Where("id=? AND source=?", item.ID, "builtin").Updates(map[string]any{
				"name": item.Name, "category": item.Category, "description": item.Description, "content": item.Content,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ListSkills(ctx context.Context) ([]models.Skill, error) {
	items := []models.Skill{}
	err := s.DB.WithContext(ctx).Where("source=?", "builtin").
		Order("FIELD(category,'发布辅助','故障排查','实施辅助','巡检诊断')").Order("id").Find(&items).Error
	return items, err
}

func (s *Store) UpdateSkill(ctx context.Context, id string, enabled bool, rules string) (*models.Skill, error) {
	var item models.Skill
	if err := s.DB.WithContext(ctx).Where("id=? AND source=?", id, "builtin").Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.ErrNotFound
		}
		return nil, err
	}
	item.Enabled, item.CustomRules, item.UpdatedAt = enabled, rules, time.Now().UTC()
	err := s.DB.WithContext(ctx).Model(&models.Skill{}).Where("id=? AND source=?", id, "builtin").
		Updates(map[string]any{"enabled": enabled, "custom_rules": rules, "updated_at": item.UpdatedAt}).Error
	return &item, err
}
