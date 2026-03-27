package service

import (
	"context"
	"fmt"

	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
)

// EvidenceService handles evidence set business logic.
type EvidenceService struct {
	evidenceRepo *repository.EvidenceRepo
	queue        *queue.Client
}

// NewEvidenceService creates a new EvidenceService.
func NewEvidenceService(evidenceRepo *repository.EvidenceRepo, q *queue.Client) *EvidenceService {
	return &EvidenceService{evidenceRepo: evidenceRepo, queue: q}
}

// GetEvidenceSet retrieves an evidence set by ID.
func (s *EvidenceService) GetEvidenceSet(ctx context.Context, id string) (*model.EvidenceSet, error) {
	es, err := s.evidenceRepo.FindEvidenceSetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("evidenceService.GetEvidenceSet: %w", err)
	}
	return es, nil
}

// RetrieveEvidence re-triggers the evidence retrieval for an evidence set.
// The actual retrieval is handled by the AI worker; we publish a queue message.
func (s *EvidenceService) RetrieveEvidence(ctx context.Context, evidenceSetID string, topK int, filterParams string) error {
	es, err := s.evidenceRepo.FindEvidenceSetByID(ctx, evidenceSetID)
	if err != nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: %w", err)
	}
	if es == nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: evidence set not found")
	}

	msg := map[string]interface{}{
		"type":          "RETRIEVE_EVIDENCE",
		"evidenceSetId": evidenceSetID,
		"topK":          topK,
		"filterParams":  filterParams,
	}
	if err := s.queue.Publish(ctx, "analysis.jobs", msg); err != nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: queue: %w", err)
	}
	return nil
}
