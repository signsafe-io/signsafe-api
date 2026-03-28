package service

import (
	"context"
	"fmt"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/cache"
	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// AnalysisService handles risk analysis business logic.
type AnalysisService struct {
	analysisRepo *repository.AnalysisRepo
	contractRepo *repository.ContractRepo
	userRepo     *repository.UserRepo
	queue        *queue.Client
	cache        *cache.Client
}

// NewAnalysisService creates a new AnalysisService.
func NewAnalysisService(
	analysisRepo *repository.AnalysisRepo,
	contractRepo *repository.ContractRepo,
	userRepo *repository.UserRepo,
	q *queue.Client,
	c *cache.Client,
) *AnalysisService {
	return &AnalysisService{
		analysisRepo: analysisRepo,
		contractRepo: contractRepo,
		userRepo:     userRepo,
		queue:        q,
		cache:        c,
	}
}

// GetLatestAnalysis returns the most recent analysis for a contract with its clause results.
// userID is used to verify that the caller is a member of the contract's organization.
func (s *AnalysisService) GetLatestAnalysis(ctx context.Context, contractID, userID string) (*model.RiskAnalysis, []repository.ClauseResultWithEvidence, error) {
	// Verify the contract exists and the caller is a member of its org.
	c, err := s.contractRepo.FindContractByID(ctx, contractID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: find contract: %w", err)
	}
	if c == nil {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: contract not found")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, c.OrganizationID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: check membership: %w", err)
	}
	if !member {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: access denied")
	}

	a, err := s.analysisRepo.FindLatestAnalysisByContractID(ctx, contractID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: %w", err)
	}
	if a == nil {
		return nil, nil, nil
	}

	results, err := s.analysisRepo.ListClauseResultsWithEvidenceByAnalysisID(ctx, a.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetLatestAnalysis: clause results: %w", err)
	}

	return a, results, nil
}

// CreateAnalysis creates a risk analysis and enqueues the job.
// userID must be a member of the contract's organization.
func (s *AnalysisService) CreateAnalysis(ctx context.Context, contractID, userID string) (string, error) {
	// Verify the contract exists and the caller is a member of its org.
	c, err := s.contractRepo.FindContractByID(ctx, contractID)
	if err != nil {
		return "", fmt.Errorf("analysisService.CreateAnalysis: find contract: %w", err)
	}
	if c == nil {
		return "", fmt.Errorf("analysisService.CreateAnalysis: contract not found")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, c.OrganizationID)
	if err != nil {
		return "", fmt.Errorf("analysisService.CreateAnalysis: check membership: %w", err)
	}
	if !member {
		return "", fmt.Errorf("analysisService.CreateAnalysis: access denied")
	}

	// Distributed lock to prevent duplicate analyses.
	lockKey := fmt.Sprintf("analysis:lock:%s", contractID)
	acquired, err := s.cache.SetNX(ctx, lockKey, userID, 10*time.Minute)
	if err != nil {
		return "", fmt.Errorf("analysisService.CreateAnalysis: lock: %w", err)
	}
	if !acquired {
		return "", fmt.Errorf("analysisService.CreateAnalysis: analysis already running for contract %s", contractID)
	}

	analysisID := util.NewID()
	a := &model.RiskAnalysis{
		ID:          analysisID,
		ContractID:  contractID,
		RequestedBy: userID,
		Status:      "pending",
	}
	if err := s.analysisRepo.CreateAnalysis(ctx, a); err != nil {
		_ = s.cache.Delete(ctx, lockKey)
		return "", fmt.Errorf("analysisService.CreateAnalysis: db: %w", err)
	}

	msg := queue.AnalysisMessage{
		ContractID: contractID,
		AnalysisID: analysisID,
	}
	if err := s.queue.Publish(ctx, "analysis.jobs", msg); err != nil {
		_ = s.cache.Delete(ctx, lockKey)
		return "", fmt.Errorf("analysisService.CreateAnalysis: queue: %w", err)
	}

	// Release the lock immediately after successful enqueue.
	// The lock only needed to prevent duplicate in-flight requests;
	// the AI worker will handle deduplication at the job level.
	_ = s.cache.Delete(ctx, lockKey)

	return analysisID, nil
}

// GetAnalysis returns a risk analysis with its clause results (including evidence set IDs).
// userID is used to verify that the caller is a member of the analysis's contract organization.
func (s *AnalysisService) GetAnalysis(ctx context.Context, analysisID, userID string) (*model.RiskAnalysis, []repository.ClauseResultWithEvidence, error) {
	a, err := s.analysisRepo.FindAnalysisByID(ctx, analysisID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: %w", err)
	}
	if a == nil {
		return nil, nil, nil
	}

	// Verify the caller belongs to the contract's org.
	c, err := s.contractRepo.FindContractByID(ctx, a.ContractID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: find contract: %w", err)
	}
	if c == nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: contract not found")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, c.OrganizationID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: check membership: %w", err)
	}
	if !member {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: access denied")
	}

	results, err := s.analysisRepo.ListClauseResultsWithEvidenceByAnalysisID(ctx, analysisID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: clause results: %w", err)
	}

	return a, results, nil
}

// allowedRiskLevels is the set of valid values for newRiskLevel in an override.
var allowedRiskLevels = map[string]struct{}{
	"HIGH":   {},
	"MEDIUM": {},
	"LOW":    {},
}

// CreateOverride stores a risk override for a clause result.
// It verifies that clauseResultID belongs to the given analysisID and
// that the caller is a member of the analysis's contract organization.
func (s *AnalysisService) CreateOverride(ctx context.Context, analysisID, clauseResultID, newRiskLevel, reason, userID string) (*model.RiskOverride, error) {
	if _, ok := allowedRiskLevels[newRiskLevel]; !ok {
		return nil, fmt.Errorf("analysisService.CreateOverride: invalid risk level %q (must be HIGH, MEDIUM, or LOW)", newRiskLevel)
	}

	// Verify the analysis exists.
	a, err := s.analysisRepo.FindAnalysisByID(ctx, analysisID)
	if err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: find analysis: %w", err)
	}
	if a == nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: analysis not found")
	}

	// Verify the caller belongs to the contract's org.
	c, err := s.contractRepo.FindContractByID(ctx, a.ContractID)
	if err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: find contract: %w", err)
	}
	if c == nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: contract not found")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, c.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: check membership: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("analysisService.CreateOverride: access denied")
	}

	cr, err := s.analysisRepo.FindClauseResultByID(ctx, clauseResultID)
	if err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: find clause result: %w", err)
	}
	if cr == nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: clause result not found")
	}

	// Verify the clause result belongs to the specified analysis.
	if cr.AnalysisID != analysisID {
		return nil, fmt.Errorf("analysisService.CreateOverride: clause result does not belong to analysis")
	}

	// Always use the AI-assessed risk level as the original, regardless of prior overrides.
	originalLevel := cr.RiskLevel

	o := &model.RiskOverride{
		ID:                util.NewID(),
		ClauseResultID:    clauseResultID,
		OriginalRiskLevel: originalLevel,
		NewRiskLevel:      newRiskLevel,
		Reason:            reason,
		DecidedBy:         userID,
	}

	if err := s.analysisRepo.CreateRiskOverride(ctx, o); err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: %w", err)
	}

	return o, nil
}
