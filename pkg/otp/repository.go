package otp

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"time"
)

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})}
}
func (r *Repository) Migrate() error {
	return r.db.AutoMigrate(&Project{}, &Challenge{}, &Job{}, &Rate{}, &Audit{}, &Receipt{})
}
func (r *Repository) projectTx(ctx context.Context, id string, fn func(*gorm.DB, *Project) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Project
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&p, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return problem(404, "NOT_FOUND", "Project not found")
			}
			return err
		}
		return fn(tx, &p)
	})
}
func audit(tx *gorm.DB, pid, action, id string) error {
	return tx.Create(&Audit{ID: uuid.NewString(), ProjectID: pid, Action: action, ResourceID: id}).Error
}
func rate(tx *gorm.DB, key string, limit, seconds int, now time.Time) error {
	window := now.Unix() / int64(seconds)
	key = fmt.Sprintf("%s:%d", key, window)
	expiry := time.Unix((window+1)*int64(seconds), 0)
	r := Rate{Key: key, Count: 1, ExpiresAt: expiry}
	if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.Assignments(map[string]interface{}{"count": gorm.Expr("otp_rates.count + 1")})}).Create(&r).Error; err != nil {
		return err
	}
	if err := tx.First(&r, "key = ?", key).Error; err != nil {
		return err
	}
	if r.Count > limit {
		return &Error{Status: 429, Code: "RATE_LIMITED", Message: "Request limit reached", RetryAfter: int(expiry.Sub(now).Seconds()) + 1}
	}
	return nil
}
