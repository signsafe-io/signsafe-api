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

	return analysisID, nil
}

// GetAnalysis returns a risk analysis with its clause results.
func (s *AnalysisService) GetAnalysis(ctx context.Context, analysisID string) (*model.RiskAnalysis, []model.ClauseResult, error) {
	a, err := s.analysisRepo.FindAnalysisByID(ctx, analysisID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: %w", err)
	}
	if a == nil {
		return nil, nil, nil
	}

	results, err := s.analysisRepo.ListClauseResultsByAnalysisID(ctx, analysisID)
	if err != nil {
		return nil, nil, fmt.Errorf("analysisService.GetAnalysis: clause results: %w", err)
	}

	return a, results, nil
}

// CreateOverride stores a risk override for a clause result.
func (s *AnalysisService) CreateOverride(ctx context.Context, analysisID, clauseResultID, newRiskLevel, reason, userID string) (*model.RiskOverride, error) {
	cr, err := s.analysisRepo.FindClauseResultByID(ctx, clauseResultID)
	if err != nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: find clause result: %w", err)
	}
	if cr == nil {
		return nil, fmt.Errorf("analysisService.CreateOverride: clause result not found")
	}

	// Use the current effective risk level as original.
	originalLevel := cr.RiskLevel
	if cr.OverriddenRiskLevel != nil {
		originalLevel = *cr.OverriddenRiskLevel
	}

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
