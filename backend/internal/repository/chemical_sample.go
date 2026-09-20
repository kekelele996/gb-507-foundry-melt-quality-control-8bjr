package repository

import (
	"context"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"gorm.io/gorm"
)

// ChemicalSampleRepository owns all persistence operations for 化验样本.
type ChemicalSampleRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.ChemicalSample], error)
	Get(context.Context, uint) (model.ChemicalSample, error)
	GetByCode(context.Context, string) (model.ChemicalSample, error)
	HasTerminalForHeat(context.Context, string) (bool, error)
	ListForHeat(context.Context, string) ([]model.ChemicalSample, error)
	ListVerifiedForHeat(context.Context, string) ([]model.ChemicalSample, error)
	LockVerified(context.Context, uint) error
	Create(context.Context, *model.ChemicalSample) error
	Update(context.Context, uint, uint, *model.ChemicalSample) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type chemicalSampleRepository struct {
	store *Store[model.ChemicalSample]
}

func NewChemicalSampleRepository(db *gorm.DB) ChemicalSampleRepository {
	return &chemicalSampleRepository{store: NewStore[model.ChemicalSample](db)}
}

func (r *chemicalSampleRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.ChemicalSample], error) {
	return r.store.List(ctx, q)
}
func (r *chemicalSampleRepository) Get(ctx context.Context, id uint) (model.ChemicalSample, error) {
	return r.store.Get(ctx, id)
}
func (r *chemicalSampleRepository) GetByCode(ctx context.Context, code string) (model.ChemicalSample, error) {
	var item model.ChemicalSample
	err := dbForContext(ctx, r.store.db).Where("code = ?", code).First(&item).Error
	return item, err
}
func (r *chemicalSampleRepository) HasTerminalForHeat(ctx context.Context, heatCode string) (bool, error) {
	var total int64
	err := dbForContext(ctx, r.store.db).Model(&model.ChemicalSample{}).
		Where("heat_code = ? AND status IN ?", heatCode, []string{"verified", "rejected", "locked"}).Count(&total).Error
	return total > 0, err
}
func (r *chemicalSampleRepository) ListForHeat(ctx context.Context, heatCode string) ([]model.ChemicalSample, error) {
	items := make([]model.ChemicalSample, 0)
	err := dbForContext(ctx, r.store.db).Where("heat_code = ?", heatCode).
		Order("sampled_at DESC, id DESC").Find(&items).Error
	return items, err
}
func (r *chemicalSampleRepository) ListVerifiedForHeat(ctx context.Context, heatCode string) ([]model.ChemicalSample, error) {
	items := make([]model.ChemicalSample, 0)
	err := dbForContext(ctx, r.store.db).Where("heat_code = ? AND status = ?", heatCode, "verified").
		Order("sampled_at DESC, id DESC").Find(&items).Error
	return items, err
}
func (r *chemicalSampleRepository) LockVerified(ctx context.Context, id uint) error {
	result := dbForContext(ctx, r.store.db).Model(&model.ChemicalSample{}).
		Where("id = ? AND status = ?", id, "verified").
		Updates(map[string]any{"status": "locked", "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}
func (r *chemicalSampleRepository) Create(ctx context.Context, item *model.ChemicalSample) error {
	return r.store.Create(ctx, item)
}
func (r *chemicalSampleRepository) Update(ctx context.Context, id, version uint, item *model.ChemicalSample) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *chemicalSampleRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *chemicalSampleRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
