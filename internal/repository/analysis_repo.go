package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// AnalysisRepo handles DB operations for risk analyses and clause results.
type AnalysisRepo struct {
	db *sqlx.DB
}

// NewAnalysisRepo creates a new AnalysisRepo.
func NewAnalysisRepo(db *sqlx.DB) *AnalysisRepo {
	return &AnalysisRepo{db: db}
}

// CreateAnalysis inserts a new risk analysis.
func (r *AnalysisRepo) CreateAnalysis(ctx context.Context, a *model.RiskAnalysis) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO risk_analyses
			(id, contract_id, requested_by, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NOW(),NOW())`,
		a.ID, a.ContractID, a.RequestedBy, a.Status)
	if err != nil {
		return fmt.Errorf("analysisRepo.CreateAnalysis: %w", err)
	}
	return nil
}

// FindAnalysisByID retrieves a risk analysis by ID.
func (r *AnalysisRepo) FindAnalysisByID(ctx context.Context, id string) (*model.RiskAnalysis, error) {
	var a model.RiskAnalysis
	err := r.db.GetContext(ctx, &a, `SELECT * FROM risk_analyses WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("analysisRepo.FindAnalysisByID: %w", err)
	}
	return &a, nil
}

// ListClauseResultsByAnalysisID returns all clause results for an analysis.
func (r *AnalysisRepo) ListClauseResultsByAnalysisID(ctx context.Context, analysisID string) ([]model.ClauseResult, error) {
	var results []model.ClauseResult
	err := r.db.SelectContext(ctx, &results,
		`SELECT * FROM clause_results WHERE analysis_id = $1 ORDER BY created_at`,
		analysisID)
	if err != nil {
		return nil, fmt.Errorf("analysisRepo.ListClauseResultsByAnalysisID: %w", err)
	}
	return results, nil
}

// ClauseResultWithEvidence extends ClauseResult with an optional evidence set ID.
type ClauseResultWithEvidence struct {
	model.ClauseResult
	EvidenceSetID *string `db:"evidence_set_id" json:"evidenceSetId"`
}

// ListClauseResultsWithEvidenceByAnalysisID returns clause results joined with
// their evidence set IDs (NULL when no evidence set exists yet).
func (r *AnalysisRepo) ListClauseResultsWithEvidenceByAnalysisID(ctx context.Context, analysisID string) ([]ClauseResultWithEvidence, error) {
	var results []ClauseResultWithEvidence
	err := r.db.SelectContext(ctx, &results, `
		SELECT cr.*,
		       es.id AS evidence_set_id
		FROM clause_results cr
		LEFT JOIN evidence_sets es ON es.clause_result_id = cr.id
		WHERE cr.analysis_id = $1
		ORDER BY cr.created_at`,
		analysisID)
	if err != nil {
		return nil, fmt.Errorf("analysisRepo.ListClauseResultsWithEvidenceByAnalysisID: %w", err)
	}
	return results, nil
}

// FindClauseResultByID retrieves a clause result by ID.
func (r *AnalysisRepo) FindClauseResultByID(ctx context.Context, id string) (*model.ClauseResult, error) {
	var cr model.ClauseResult
	err := r.db.GetContext(ctx, &cr, `SELECT * FROM clause_results WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("analysisRepo.FindClauseResultByID: %w", err)
	}
	return &cr, nil
}

// CreateRiskOverride inserts a risk override and updates clause_results.
func (r *AnalysisRepo) CreateRiskOverride(ctx context.Context, o *model.RiskOverride) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("analysisRepo.CreateRiskOverride: begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO risk_overrides
			(id, clause_result_id, original_risk_level, new_risk_level, reason, decided_by, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW())`,
		o.ID, o.ClauseResultID, o.OriginalRiskLevel, o.NewRiskLevel, o.Reason, o.DecidedBy)
	if err != nil {
		return fmt.Errorf("analysisRepo.CreateRiskOverride: insert override: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE clause_results
		SET overridden_risk_level = $1,
		    override_reason = $2,
		    overridden_by = $3,
		    overridden_at = NOW(),
		    updated_at = NOW()
		WHERE id = $4`,
		o.NewRiskLevel, o.Reason, o.DecidedBy, o.ClauseResultID)
	if err != nil {
		return fmt.Errorf("analysisRepo.CreateRiskOverride: update clause_result: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("analysisRepo.CreateRiskOverride: commit: %w", err)
	}
	return nil
}
