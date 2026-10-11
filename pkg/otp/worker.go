package otp

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"log"
	"time"
)

// ProcessOne claims a job durably before making any external request. A crash after
// claiming is ambiguous: recovery marks UNKNOWN, never blindly resends a code.
func (s *Service) ProcessOne(ctx context.Context) (bool, error) {
	var job Job
	var challenge Challenge
	var project Project
	err := s.repo.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := s.now().UTC()
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("state = ? AND available_at <= ?", "QUEUED", now).Order("available_at").First(&job).Error; err != nil {
			return err
		}
		if err := tx.First(&challenge, "id = ?", job.ChallengeID).Error; err != nil {
			return err
		}
		if err := tx.First(&project, "id = ?", job.ProjectID).Error; err != nil {
			return err
		}
		if !project.Enabled || challenge.State != "ACTIVE" || !now.Before(challenge.ExpiresAt) {
			return tx.Model(&job).Updates(map[string]interface{}{"state": "EXPIRED", "payload": ""}).Error
		}
		job.State = "SENDING"
		job.Attempts++
		job.StartedAt = &now
		return tx.Save(&job).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if job.State != "SENDING" {
		return true, nil
	}
	code, codeErr := s.open(job.Payload, "code:"+challenge.ID)
	phone, phoneErr := s.open(challenge.PhoneCipher, "phone:"+challenge.ID)
	result := SendResult{State: "FAILED", ErrorCode: "PAYLOAD_UNAVAILABLE"}
	if codeErr == nil && phoneErr == nil {
		sendCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		result = s.sender.Send(sendCtx, Delivery{project, challenge.ID, phone, code})
		cancel()
	}
	err = s.repo.projectTx(ctx, project.ID, func(tx *gorm.DB, current *Project) error {
		var c Challenge
		if err := tx.First(&c, "id = ?", challenge.ID).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{"state": result.State, "provider_id": result.ProviderID, "error_code": result.ErrorCode, "payload": ""}
		if result.Retry && job.Attempts < 3 && current.Enabled && c.State == "ACTIVE" && s.now().Add(30*time.Second).Before(c.ExpiresAt) {
			updates["state"] = "QUEUED"
			updates["payload"] = job.Payload
			updates["available_at"] = s.now().Add(time.Duration(job.Attempts) * 30 * time.Second)
		} else if result.Retry {
			updates["state"] = "FAILED"
		}
		if result.State == "SIMULATED" && current.Enabled && c.State == "ACTIVE" && s.now().Before(c.ExpiresAt) {
			updates["payload"] = job.Payload
		}
		if err := tx.Model(&Job{}).Where("id = ? AND state IN ?", job.ID, []string{"SENDING", "UNKNOWN"}).Updates(updates).Error; err != nil {
			return err
		}
		return audit(tx, project.ID, "delivery."+result.State, challenge.ID)
	})
	if err == nil {
		err = s.ReconcileReceipts(ctx)
	}
	return true, err
}
func (s *Service) Cleanup(ctx context.Context) error {
	now := s.now().UTC()
	var projects []Project
	if err := s.repo.db.WithContext(ctx).Find(&projects).Error; err != nil {
		return err
	}
	for _, project := range projects {
		if err := s.repo.projectTx(ctx, project.ID, func(tx *gorm.DB, p *Project) error {
			if err := tx.Model(&Job{}).Where("project_id = ? AND state = ? AND started_at < ?", p.ID, "SENDING", now.Add(-2*time.Minute)).Updates(map[string]interface{}{"state": "UNKNOWN", "payload": "", "error_code": "WORKER_INTERRUPTED"}).Error; err != nil {
				return err
			}
			expired := tx.Model(&Challenge{}).Select("id").Where("project_id = ? AND expires_at <= ?", p.ID, now)
			if err := tx.Model(&Job{}).Where("challenge_id IN (?) AND payload <> ?", expired, "").Update("payload", "").Error; err != nil {
				return err
			}
			if err := tx.Model(&Job{}).Where("state IN ? AND challenge_id IN (?)", []string{"QUEUED", "SIMULATED"}, expired).Update("state", "EXPIRED").Error; err != nil {
				return err
			}
			if err := tx.Model(&Challenge{}).Where("project_id = ? AND state = ? AND expires_at <= ?", p.ID, "ACTIVE", now).Updates(map[string]interface{}{"state": "EXPIRED", "digest": ""}).Error; err != nil {
				return err
			}
			if err := tx.Model(&Challenge{}).Where("project_id = ? AND expires_at < ? AND (phone_cipher <> ? OR digest <> ?)", p.ID, now.Add(-time.Hour), "", "").Updates(map[string]interface{}{"phone_cipher": "", "digest": ""}).Error; err != nil {
				return err
			}
			cutoff := now.Add(-time.Duration(s.cfg.RetentionDays) * 24 * time.Hour)
			old := tx.Model(&Challenge{}).Select("id").Where("project_id = ? AND created_at < ?", p.ID, cutoff)
			if err := tx.Where("challenge_id IN (?)", old).Delete(&Job{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ? AND created_at < ?", p.ID, cutoff).Delete(&Challenge{}).Error; err != nil {
				return err
			}
			return tx.Where("project_id = ? AND created_at < ?", p.ID, cutoff).Delete(&Audit{}).Error
		}); err != nil {
			return err
		}
	}
	if err := s.ReconcileReceipts(ctx); err != nil {
		return err
	}
	if err := s.repo.db.WithContext(ctx).Where("created_at < ?", now.Add(-time.Duration(s.cfg.RetentionDays)*24*time.Hour)).Delete(&Receipt{}).Error; err != nil {
		return err
	}
	return s.repo.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&Rate{}).Error
}
func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	lastCleanup := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if time.Since(lastCleanup) > time.Minute {
				if err := s.Cleanup(ctx); err != nil {
					log.Print("OTP cleanup failed; check database health")
				}
				lastCleanup = time.Now()
			}
			for i := 0; i < 10; i++ {
				worked, err := s.ProcessOne(ctx)
				if err != nil {
					if ctx.Err() == nil {
						log.Print("OTP worker failed; check database health")
					}
					break
				}
				if !worked {
					break
				}
			}
		}
	}
}
