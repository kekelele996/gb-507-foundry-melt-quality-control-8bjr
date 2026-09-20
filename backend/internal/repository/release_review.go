package repository

import (
	"context"
	"time"

	"github.com/blueship581/foundry-melt-quality-control/backend/internal/dto"
	"github.com/blueship581/foundry-melt-quality-control/backend/internal/model"
	"gorm.io/gorm"
)

// HeatReleaseReviewRepository owns persistence for 炉次放行合议.
type HeatReleaseReviewRepository interface {
	List(context.Context, dto.PageQuery) (Page[model.HeatReleaseReview], error)
	Get(context.Context, uint) (model.HeatReleaseReview, error)
	FindForHeat(context.Context, string) (model.HeatReleaseReview, error)
	Create(context.Context, *model.HeatReleaseReview) error
	// ClaimOpen performs the compare-and-set that makes concurrent verdicts
	// succeed at most once: only an "open" review can be claimed.
	ClaimOpen(context.Context, uint, uint, string) error
	// WriteVerdictMeta persists verdict columns after a successful claim.
	WriteVerdictMeta(ctx context.Context, id, version uint, reviewer, reason string, decidedAt time.Time) error
	Delete(context.Context, uint) error
}

type heatReleaseReviewRepository struct {
	db *gorm.DB
}

func NewHeatReleaseReviewRepository(db *gorm.DB) HeatReleaseReviewRepository {
	return &heatReleaseReviewRepository{db: db}
}

func (r *heatReleaseReviewRepository) List(ctx context.Context, q dto.PageQuery) (Page[model.HeatReleaseReview], error) {
	page, pageSize := normalizePage(q.Page, q.PageSize)
	db := dbForContext(ctx, r.db).Model(&model.HeatReleaseReview{})
	if search := q.Search; search != "" {
		wildcard := "%" + search + "%"
		db = db.Where("LOWER(code) LIKE ? OR LOWER(name) LIKE ? OR LOWER(heat_code) LIKE ?", wildcard, wildcard, wildcard)
	}
	if status := q.Status; status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return Page[model.HeatReleaseReview]{}, err
	}
	items := make([]model.HeatReleaseReview, 0)
	err := db.Order("updated_at DESC, id DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error
	return Page[model.HeatReleaseReview]{Items: items, Total: total, Page: page, PageSize: pageSize}, err
}

func (r *heatReleaseReviewRepository) Get(ctx context.Context, id uint) (model.HeatReleaseReview, error) {
	var item model.HeatReleaseReview
	err := dbForContext(ctx, r.db).First(&item, id).Error
	return item, err
}

func (r *heatReleaseReviewRepository) FindForHeat(ctx context.Context, heatCode string) (model.HeatReleaseReview, error) {
	var item model.HeatReleaseReview
	err := dbForContext(ctx, r.db).Where("heat_code = ?", heatCode).First(&item).Error
	return item, err
}

func (r *heatReleaseReviewRepository) Create(ctx context.Context, item *model.HeatReleaseReview) error {
	return dbForContext(ctx, r.db).Create(item).Error
}

func (r *heatReleaseReviewRepository) ClaimOpen(ctx context.Context, id, expectedVersion uint, target string) error {
	now := time.Now().UTC()
	result := dbForContext(ctx, r.db).Model(&model.HeatReleaseReview{}).
		Where("id = ? AND status = ? AND version = ?", id, "open", expectedVersion).
		Updates(map[string]any{"status": target, "version": gorm.Expr("version + 1"), "updated_at": now, "decided_at": now})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *heatReleaseReviewRepository) WriteVerdictMeta(ctx context.Context, id, version uint, reviewer, reason string, decidedAt time.Time) error {
	result := dbForContext(ctx, r.db).Model(&model.HeatReleaseReview{}).
		Where("id = ? AND version = ?", id, version).
		Updates(map[string]any{"reviewer": reviewer, "verdict_reason": reason, "decided_at": decidedAt, "updated_at": decidedAt})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (r *heatReleaseReviewRepository) Delete(ctx context.Context, id uint) error {
	result := dbForContext(ctx, r.db).Delete(&model.HeatReleaseReview{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
