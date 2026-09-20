package repository

import (
	"context"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"gorm.io/gorm"
)

// QualityDecisionRepository owns all persistence operations for 质量决定.
type QualityDecisionRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.QualityDecision], error)
	Get(context.Context, uint) (model.QualityDecision, error)
	GetByCode(context.Context, string) (model.QualityDecision, error)
	HasForHeat(context.Context, string, uint) (bool, error)
	Create(context.Context, *model.QualityDecision) error
	Update(context.Context, uint, uint, *model.QualityDecision) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type qualityDecisionRepository struct {
	store *Store[model.QualityDecision]
}

func NewQualityDecisionRepository(db *gorm.DB) QualityDecisionRepository {
	return &qualityDecisionRepository{store: NewStore[model.QualityDecision](db)}
}

func (r *qualityDecisionRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.QualityDecision], error) {
	return r.store.List(ctx, q)
}
func (r *qualityDecisionRepository) Get(ctx context.Context, id uint) (model.QualityDecision, error) {
	return r.store.Get(ctx, id)
}
func (r *qualityDecisionRepository) GetByCode(ctx context.Context, code string) (model.QualityDecision, error) {
	var item model.QualityDecision
	err := dbForContext(ctx, r.store.db).Where("code = ?", code).First(&item).Error
	return item, err
}
func (r *qualityDecisionRepository) HasForHeat(ctx context.Context, heatCode string, excludeID uint) (bool, error) {
	var total int64
	db := dbForContext(ctx, r.store.db).Model(&model.QualityDecision{}).
		Where("heat_code = ?", heatCode)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	err := db.Count(&total).Error
	return total > 0, err
}
func (r *qualityDecisionRepository) Create(ctx context.Context, item *model.QualityDecision) error {
	return r.store.Create(ctx, item)
}
func (r *qualityDecisionRepository) Update(ctx context.Context, id, version uint, item *model.QualityDecision) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *qualityDecisionRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *qualityDecisionRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
