package service

import (
	"context"
	"fmt"

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
