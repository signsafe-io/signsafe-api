package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// AuditRepo handles DB operations for audit events.
type AuditRepo struct {
	db *sqlx.DB
}

// NewAuditRepo creates a new AuditRepo.
func NewAuditRepo(db *sqlx.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// CreateAuditEvent inserts a new audit event.
func (r *AuditRepo) CreateAuditEvent(ctx context.Context, e *model.AuditEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events
			(id, actor_id, actor_email, action, target_type, target_id,
			 organization_id, context, ip_address, user_agent, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,NOW())`,
		e.ID, e.ActorID, e.ActorEmail, e.Action, e.TargetType, e.TargetID,
		e.OrganizationID, e.Context, e.IPAddress, e.UserAgent)
	if err != nil {
		return fmt.Errorf("auditRepo.CreateAuditEvent: %w", err)
	}
	return nil
}
