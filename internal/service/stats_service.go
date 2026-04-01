package service

import (
	"context"
	"fmt"

	"github.com/signsafe-io/signsafe-api/internal/repository"
)

// ExpiryBuckets holds contract counts expiring within 30, 60, and 90 days.
type ExpiryBuckets struct {
	Days30 int `json:"days30"`
	Days60 int `json:"days60"`
	Days90 int `json:"days90"`
}

// DashboardStats is the full response for the dashboard statistics endpoint.
type DashboardStats struct {
	TotalContracts      int                         `json:"totalContracts"`
	UploadedContracts   int                         `json:"uploadedContracts"`
	ProcessingContracts int                         `json:"processingContracts"`
	ReadyContracts      int                         `json:"readyContracts"`
	FailedContracts     int                         `json:"failedContracts"`
	RecentAnalyses      int                         `json:"recentAnalyses"`
	ExpiringSoon        int                         `json:"expiringSoon"` // = ExpiryBuckets.Days30, kept for backward compat
	ExpiryBuckets       ExpiryBuckets               `json:"expiryBuckets"`
	RiskDistribution    repository.RiskDistribution `json:"riskDistribution"`
	RecentContracts     []repository.RecentContract `json:"recentContracts"`
}

// StatsService provides aggregated statistics for organizations.
type StatsService struct {
	statsRepo *repository.StatsRepo
	userRepo  *repository.UserRepo
}

// NewStatsService creates a new StatsService.
func NewStatsService(statsRepo *repository.StatsRepo, userRepo *repository.UserRepo) *StatsService {
	return &StatsService{statsRepo: statsRepo, userRepo: userRepo}
}

// GetDashboardStats returns aggregated statistics for the given organization.
// It verifies that the requesting user is a member of the org before querying.
func (s *StatsService) GetDashboardStats(ctx context.Context, userID, orgID string) (*DashboardStats, error) {
	// Verify membership.
	isMember, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("statsService.GetDashboardStats: check membership: %w", err)
	}
	if !isMember {
		return nil, ErrAccessDenied
	}

	// Fetch contract counts + recent analyses count.
	orgStats, err := s.statsRepo.GetOrgStats(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("statsService.GetDashboardStats: org stats: %w", err)
	}

	// Fetch risk distribution.
	riskDist, err := s.statsRepo.GetRiskDistribution(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("statsService.GetDashboardStats: risk distribution: %w", err)
	}

	// Fetch recent 5 contracts.
	recentContracts, err := s.statsRepo.ListRecentContracts(ctx, orgID, 5)
	if err != nil {
		return nil, fmt.Errorf("statsService.GetDashboardStats: recent contracts: %w", err)
	}

	return &DashboardStats{
		TotalContracts:      orgStats.TotalContracts,
		UploadedContracts:   orgStats.UploadedContracts,
		ProcessingContracts: orgStats.ProcessingContracts,
		ReadyContracts:      orgStats.ReadyContracts,
		FailedContracts:     orgStats.FailedContracts,
		RecentAnalyses:      orgStats.RecentAnalyses,
		ExpiringSoon:        orgStats.ExpiringSoon,
		ExpiryBuckets: ExpiryBuckets{
			Days30: orgStats.Expiring30,
			Days60: orgStats.Expiring60,
			Days90: orgStats.Expiring90,
		},
		RiskDistribution: *riskDist,
		RecentContracts:  recentContracts,
	}, nil
}
