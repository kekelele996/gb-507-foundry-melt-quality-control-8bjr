package repository

import (
	"context"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"gorm.io/gorm"
)

// HeatRepository owns all persistence operations for 炉次.
type HeatRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.Heat], error)
	Get(context.Context, uint) (model.Heat, error)
	GetByCode(context.Context, string) (model.Heat, error)
	LockByCode(context.Context, string) (model.Heat, error)
	ListByStatuses(context.Context, []string) ([]model.Heat, error)
	Create(context.Context, *model.Heat) error
	Update(context.Context, uint, uint, *model.Heat) error
	Delete(context.Context, uint) error
	CountByStatus(context.Context) (map[string]int64, error)
}

type heatRepository struct {
	store *Store[model.Heat]
}

func NewHeatRepository(db *gorm.DB) HeatRepository {
	return &heatRepository{store: NewStore[model.Heat](db)}
}

func (r *heatRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.Heat], error) {
	return r.store.List(ctx, q)
}
func (r *heatRepository) Get(ctx context.Context, id uint) (model.Heat, error) {
	return r.store.Get(ctx, id)
}
func (r *heatRepository) GetByCode(ctx context.Context, code string) (model.Heat, error) {
	var item model.Heat
	err := dbForContext(ctx, r.store.db).Where("code = ?", code).First(&item).Error
	return item, err
}

// LockByCode re-reads a heat inside the adjudication transaction. Serialization
// of concurrent adjudications is guaranteed by the unique index on
// quality_decisions.heat_code (and, on PostgreSQL, by row locks taken during
// the versioned heat update), so no dialect-specific FOR UPDATE is needed.
func (r *heatRepository) LockByCode(ctx context.Context, code string) (model.Heat, error) {
	var item model.Heat
	err := dbForContext(ctx, r.store.db).Where("code = ?", code).First(&item).Error
	return item, err
}
func (r *heatRepository) ListByStatuses(ctx context.Context, statuses []string) ([]model.Heat, error) {
	items := make([]model.Heat, 0)
	err := dbForContext(ctx, r.store.db).Where("status IN ?", statuses).
		Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}
func (r *heatRepository) Create(ctx context.Context, item *model.Heat) error {
	return r.store.Create(ctx, item)
}
func (r *heatRepository) Update(ctx context.Context, id, version uint, item *model.Heat) error {
	return r.store.Update(ctx, id, version, item)
}
func (r *heatRepository) Delete(ctx context.Context, id uint) error {
	return r.store.Delete(ctx, id)
}
func (r *heatRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	return r.store.CountByStatus(ctx)
}
