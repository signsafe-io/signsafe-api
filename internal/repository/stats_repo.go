package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// OrgStats holds aggregated statistics for an organization.
type OrgStats struct {
	TotalContracts      int `db:"total_contracts"      json:"totalContracts"`
	UploadedContracts   int `db:"uploaded_contracts"   json:"uploadedContracts"`
	ProcessingContracts int `db:"processing_contracts" json:"processingContracts"`
	ReadyContracts      int `db:"ready_contracts"      json:"readyContracts"`
	FailedContracts     int `db:"failed_contracts"     json:"failedContracts"`
	RecentAnalyses      int `db:"recent_analyses"      json:"recentAnalyses"` // last 30 days
	ExpiringSoon        int `db:"expiring_soon"        json:"expiringSoon"`   // expires within 30 days (= Expiring30)
	Expiring30          int `db:"expiring_30"          json:"expiring30"`     // expires within 30 days
	Expiring60          int `db:"expiring_60"          json:"expiring60"`     // expires within 60 days
	Expiring90          int `db:"expiring_90"          json:"expiring90"`     // expires within 90 days
}

// RiskDistribution holds the count of clause results at each risk level for an org.
type RiskDistribution struct {
	HighCount   int `db:"high_count"   json:"highCount"`
	MediumCount int `db:"medium_count" json:"mediumCount"`
	LowCount    int `db:"low_count"    json:"lowCount"`
}

// RecentContract is a lightweight summary of a contract for the dashboard.
type RecentContract struct {
	ID        string `db:"id"         json:"id"`
	Title     string `db:"title"      json:"title"`
	Status    string `db:"status"     json:"status"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

// StatsRepo handles aggregated statistics queries.
type StatsRepo struct {
	db *sqlx.DB
}

// NewStatsRepo creates a new StatsRepo.
func NewStatsRepo(db *sqlx.DB) *StatsRepo {
	return &StatsRepo{db: db}
}

// GetOrgStats returns contract counts and recent analysis count for an org.
// Expiry buckets count contracts expiring within 30/60/90 days from now.
// Already-expired contracts are excluded from all buckets.
func (r *StatsRepo) GetOrgStats(ctx context.Context, orgID string) (*OrgStats, error) {
	var s OrgStats
	err := r.db.GetContext(ctx, &s, `
		SELECT
			COUNT(*)                                                         AS total_contracts,
			COUNT(*) FILTER (WHERE status = 'uploaded')                     AS uploaded_contracts,
			COUNT(*) FILTER (WHERE status = 'processing')                   AS processing_contracts,
			COUNT(*) FILTER (WHERE status = 'ready')                        AS ready_contracts,
			COUNT(*) FILTER (WHERE status = 'failed')                       AS failed_contracts,
			(
				SELECT COUNT(*)
				FROM risk_analyses ra
				JOIN contracts c2 ON c2.id = ra.contract_id
				WHERE c2.organization_id = $1
				  AND ra.created_at >= NOW() - INTERVAL '30 days'
			)                                                                AS recent_analyses,
			COUNT(*) FILTER (
				WHERE expires_at IS NOT NULL
				  AND expires_at > NOW()
				  AND expires_at <= NOW() + INTERVAL '30 days'
			)                                                                AS expiring_soon,
			COUNT(*) FILTER (
				WHERE expires_at IS NOT NULL
				  AND expires_at > NOW()
				  AND expires_at <= NOW() + INTERVAL '30 days'
			)                                                                AS expiring_30,
			COUNT(*) FILTER (
				WHERE expires_at IS NOT NULL
				  AND expires_at > NOW()
				  AND expires_at <= NOW() + INTERVAL '60 days'
			)                                                                AS expiring_60,
			COUNT(*) FILTER (
				WHERE expires_at IS NOT NULL
				  AND expires_at > NOW()
				  AND expires_at <= NOW() + INTERVAL '90 days'
			)                                                                AS expiring_90
		FROM contracts
		WHERE organization_id = $1`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("statsRepo.GetOrgStats: %w", err)
	}
	return &s, nil
}

// GetRiskDistribution returns the count of HIGH/MEDIUM/LOW clause results from
// completed analyses for contracts belonging to the org.
func (r *StatsRepo) GetRiskDistribution(ctx context.Context, orgID string) (*RiskDistribution, error) {
	var d RiskDistribution
	err := r.db.GetContext(ctx, &d, `
		SELECT
			COUNT(*) FILTER (WHERE cr.risk_level = 'HIGH')   AS high_count,
			COUNT(*) FILTER (WHERE cr.risk_level = 'MEDIUM') AS medium_count,
			COUNT(*) FILTER (WHERE cr.risk_level = 'LOW')    AS low_count
		FROM clause_results cr
		JOIN risk_analyses ra ON ra.id = cr.analysis_id
		JOIN contracts c      ON c.id  = ra.contract_id
		WHERE c.organization_id = $1
		  AND ra.status = 'completed'`,
		orgID)
	if err != nil {
		return nil, fmt.Errorf("statsRepo.GetRiskDistribution: %w", err)
	}
	return &d, nil
}

// ListRecentContracts returns the most recent contracts for an org.
func (r *StatsRepo) ListRecentContracts(ctx context.Context, orgID string, limit int) ([]RecentContract, error) {
	var contracts []RecentContract
	err := r.db.SelectContext(ctx, &contracts, `
		SELECT id, title, status, created_at::text
		FROM contracts
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		orgID, limit)
	if err != nil {
		return nil, fmt.Errorf("statsRepo.ListRecentContracts: %w", err)
	}
	return contracts, nil
}
