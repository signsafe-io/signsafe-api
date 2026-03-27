package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// EvidenceRepo handles DB operations for evidence sets.
type EvidenceRepo struct {
	db *sqlx.DB
}

// NewEvidenceRepo creates a new EvidenceRepo.
func NewEvidenceRepo(db *sqlx.DB) *EvidenceRepo {
	return &EvidenceRepo{db: db}
}

// FindEvidenceSetByID retrieves an evidence set by ID.
func (r *EvidenceRepo) FindEvidenceSetByID(ctx context.Context, id string) (*model.EvidenceSet, error) {
	var es model.EvidenceSet
	err := r.db.GetContext(ctx, &es, `SELECT * FROM evidence_sets WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("evidenceRepo.FindEvidenceSetByID: %w", err)
	}
	return &es, nil
}

// CreateEvidenceSet inserts a new evidence set.
func (r *EvidenceRepo) CreateEvidenceSet(ctx context.Context, es *model.EvidenceSet) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO evidence_sets
			(id, clause_result_id, rationale, citations, recommended_actions,
			 top_k, filter_params, retrieved_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NOW(),NOW(),NOW())`,
		es.ID, es.ClauseResultID, es.Rationale, es.Citations, es.RecommendedActions,
		es.TopK, es.FilterParams)
	if err != nil {
		return fmt.Errorf("evidenceRepo.CreateEvidenceSet: %w", err)
	}
	return nil
}
