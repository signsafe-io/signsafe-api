package service

import (
	"context"
	"fmt"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// AuditService handles audit event business logic.
type AuditService struct {
	auditRepo *repository.AuditRepo
}

// NewAuditService creates a new AuditService.
func NewAuditService(auditRepo *repository.AuditRepo) *AuditService {
	return &AuditService{auditRepo: auditRepo}
}

// CreateAuditEventRequest holds audit event creation parameters.
type CreateAuditEventRequest struct {
	ActorID        *string
	ActorEmail     *string
	Action         string
	TargetType     *string
	TargetID       *string
	OrganizationID *string
	Context        string
	IPAddress      *string
	UserAgent      *string
}

// CreateAuditEvent inserts a new audit event.
func (s *AuditService) CreateAuditEvent(ctx context.Context, req CreateAuditEventRequest) (*model.AuditEvent, error) {
	if req.Context == "" {
		req.Context = "{}"
	}

	e := &model.AuditEvent{
		ID:             util.NewID(),
		ActorID:        req.ActorID,
		ActorEmail:     req.ActorEmail,
		Action:         req.Action,
		TargetType:     req.TargetType,
		TargetID:       req.TargetID,
		OrganizationID: req.OrganizationID,
		Context:        req.Context,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
	}

	if err := s.auditRepo.CreateAuditEvent(ctx, e); err != nil {
		return nil, fmt.Errorf("auditService.CreateAuditEvent: %w", err)
	}
	return e, nil
}

// ListAuditEventsRequest holds parameters for listing audit events.
type ListAuditEventsRequest struct {
	OrganizationID string
	Action         string
	From           *time.Time
	To             *time.Time
	Page           int
	PageSize       int
}

// ListAuditEventsResponse is returned by ListAuditEvents.
type ListAuditEventsResponse struct {
	Events   []*model.AuditEvent `json:"events"`
	Total    int                 `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"pageSize"`
}

// ListAuditEvents returns a filtered, paginated list of audit events.
func (s *AuditService) ListAuditEvents(ctx context.Context, req ListAuditEventsRequest) (*ListAuditEventsResponse, error) {
	if req.Page < 1 {
		req.Page = 1
	}
	if req.PageSize < 1 || req.PageSize > 100 {
		req.PageSize = 30
	}

	result, err := s.auditRepo.ListAuditEvents(ctx, repository.ListAuditEventsFilter{
		OrganizationID: req.OrganizationID,
		Action:         req.Action,
		From:           req.From,
		To:             req.To,
		Page:           req.Page,
		PageSize:       req.PageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("auditService.ListAuditEvents: %w", err)
	}

	return &ListAuditEventsResponse{
		Events:   result.Events,
		Total:    result.Total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}
