package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

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

// ListAuditEventsFilter holds optional filter parameters for listing audit events.
type ListAuditEventsFilter struct {
	OrganizationID string
	Action         string
	From           *time.Time
	To             *time.Time
	Page           int
	PageSize       int
}

// ListAuditEventsResult holds a page of audit events and total count.
type ListAuditEventsResult struct {
	Events []*model.AuditEvent
	Total  int
}

// ListAuditEvents returns a paginated list of audit events for an organization.
func (r *AuditRepo) ListAuditEvents(ctx context.Context, f ListAuditEventsFilter) (*ListAuditEventsResult, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 30
	}
	offset := (f.Page - 1) * f.PageSize

	// Build dynamic WHERE clause.
	conditions := []string{"organization_id = $1"}
	args := []any{f.OrganizationID}
	idx := 2

	if f.Action != "" {
		conditions = append(conditions, fmt.Sprintf("action = $%d", idx))
		args = append(args, f.Action)
		idx++
	}
	if f.From != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, *f.From)
		idx++
	}
	if f.To != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", idx))
		args = append(args, *f.To)
		idx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total matching events.
	var total int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM audit_events %s", where)
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("auditRepo.ListAuditEvents count: %w", err)
	}

	// Fetch page.
	listQ := fmt.Sprintf(`
		SELECT id, actor_id, actor_email, action, target_type, target_id,
		       organization_id, context, ip_address, user_agent, created_at
		FROM audit_events
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, idx, idx+1)

	args = append(args, f.PageSize, offset)

	var events []*model.AuditEvent
	if err := r.db.SelectContext(ctx, &events, listQ, args...); err != nil {
		return nil, fmt.Errorf("auditRepo.ListAuditEvents select: %w", err)
	}
	if events == nil {
		events = []*model.AuditEvent{}
	}

	return &ListAuditEventsResult{Events: events, Total: total}, nil
}
