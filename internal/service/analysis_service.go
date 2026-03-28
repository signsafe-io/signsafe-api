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
	queue        *queue.Client
	cache        *cache.Client
}

// NewAnalysisService creates a new AnalysisService.
func NewAnalysisService(
	analysisRepo *repository.AnalysisRepo,
	contractRepo *repository.ContractRepo,
	q *queue.Client,
	c *cache.Client,
) *AnalysisService {
	return &AnalysisService{
		analysisRepo: analysisRepo,
		contractRepo: contractRepo,
		queue:        q,
		cache:        c,
	}
}

// GetLatestAnalysis returns the most recent analysis for a contract with its clause results.
func (s *AnalysisService) GetLatestAnalysis(ctx context.Context, contractID string) (*model.RiskAnalysis, []repository.ClauseResultWithEvidence, error) {
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
func (s *AnalysisService) CreateAnalysis(ctx context.Context, contractID, userID string) (string, error) {
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
func (s *AnalysisService) GetAnalysis(ctx context.Context, analysisID string) (*model.RiskAnalysis, []repository.ClauseResultWithEvidence, error) {
	a, err := s.analysisRepo.FindAnalysisByID(ctx, analysisID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: %w", err)
	}
	if a == nil {
		return nil, nil, nil
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
// It verifies that clauseResultID belongs to the given analysisID.
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
