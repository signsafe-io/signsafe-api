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
	analysisRepo *repository.AnalysisRepo
	contractRepo *repository.ContractRepo
	userRepo     *repository.UserRepo
	queue        *queue.Client
}

// NewEvidenceService creates a new EvidenceService.
func NewEvidenceService(
	evidenceRepo *repository.EvidenceRepo,
	analysisRepo *repository.AnalysisRepo,
	contractRepo *repository.ContractRepo,
	userRepo *repository.UserRepo,
	q *queue.Client,
) *EvidenceService {
	return &EvidenceService{
		evidenceRepo: evidenceRepo,
		analysisRepo: analysisRepo,
		contractRepo: contractRepo,
		userRepo:     userRepo,
		queue:        q,
	}
}

// checkEvidenceSetAccess verifies the caller is a member of the organization
// that owns the contract associated with the evidence set.
func (s *EvidenceService) checkEvidenceSetAccess(ctx context.Context, es *model.EvidenceSet, userID string) error {
	// evidence_set → clause_result → analysis → contract → org
	cr, err := s.analysisRepo.FindClauseResultByID(ctx, es.ClauseResultID)
	if err != nil {
		return fmt.Errorf("find clause result: %w", err)
	}
	if cr == nil {
		return fmt.Errorf("clause result not found")
	}

	a, err := s.analysisRepo.FindAnalysisByID(ctx, cr.AnalysisID)
	if err != nil {
		return fmt.Errorf("find analysis: %w", err)
	}
	if a == nil {
		return fmt.Errorf("analysis not found")
	}

	c, err := s.contractRepo.FindContractByID(ctx, a.ContractID)
	if err != nil {
		return fmt.Errorf("find contract: %w", err)
	}
	if c == nil {
		return fmt.Errorf("contract not found")
	}

	member, err := s.userRepo.IsOrgMember(ctx, userID, c.OrganizationID)
	if err != nil {
		return fmt.Errorf("check membership: %w", err)
	}
	if !member {
		return fmt.Errorf("access denied")
	}
	return nil
}

// GetEvidenceSet retrieves an evidence set by ID.
// userID must be a member of the evidence set's contract organization.
func (s *EvidenceService) GetEvidenceSet(ctx context.Context, id, userID string) (*model.EvidenceSet, error) {
	es, err := s.evidenceRepo.FindEvidenceSetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("evidenceService.GetEvidenceSet: %w", err)
	}
	if es == nil {
		return nil, nil
	}

	if err := s.checkEvidenceSetAccess(ctx, es, userID); err != nil {
		return nil, fmt.Errorf("evidenceService.GetEvidenceSet: %w", err)
	}

	return es, nil
}

// RetrieveEvidence re-triggers the evidence retrieval for an evidence set.
// The actual retrieval is handled by the AI worker; we publish a queue message.
// userID must be a member of the evidence set's contract organization.
func (s *EvidenceService) RetrieveEvidence(ctx context.Context, evidenceSetID string, topK int, filterParams string, userID string) error {
	es, err := s.evidenceRepo.FindEvidenceSetByID(ctx, evidenceSetID)
	if err != nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: %w", err)
	}
	if es == nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: evidence set not found")
	}

	if err := s.checkEvidenceSetAccess(ctx, es, userID); err != nil {
		return fmt.Errorf("evidenceService.RetrieveEvidence: %w", err)
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
