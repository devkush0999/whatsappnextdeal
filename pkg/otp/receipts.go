package otp

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

func (s *Service) RecordReceipt(ctx context.Context, id, state string) error {
	ranks := map[string]int{"failed": 1, "delivered": 2, "read": 3}
	rank, ok := ranks[state]
	if !ok || len(id) == 0 || len(id) > 256 {
		return nil
	}
	return s.repo.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "provider_id"}}, DoUpdates: clause.Assignments(map[string]interface{}{"rank": gorm.Expr("GREATEST(otp_receipts.rank, EXCLUDED.rank)"), "state": gorm.Expr("CASE WHEN EXCLUDED.rank > otp_receipts.rank THEN EXCLUDED.state ELSE otp_receipts.state END")})}).Create(&Receipt{ProviderID: id, State: strings.ToUpper(state), Rank: rank}).Error
}
func (s *Service) ReconcileReceipts(ctx context.Context) error {
	return s.repo.db.WithContext(ctx).Exec(`UPDATE otp_jobs AS j SET state = r.state FROM otp_receipts AS r
 WHERE j.provider = 'meta' AND j.provider_id = r.provider_id AND
 ((r.state = 'READ' AND j.state IN ('ACCEPTED','UNKNOWN','FAILED','DELIVERED')) OR
  (r.state = 'DELIVERED' AND j.state IN ('ACCEPTED','UNKNOWN','FAILED')) OR
  (r.state = 'FAILED' AND j.state IN ('ACCEPTED','UNKNOWN')))`).Error
}
