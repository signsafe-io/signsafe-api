package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// ContractRepo handles DB operations for contracts, ingestion jobs, and clauses.
type ContractRepo struct {
	db *sqlx.DB
}

// NewContractRepo creates a new ContractRepo.
func NewContractRepo(db *sqlx.DB) *ContractRepo {
	return &ContractRepo{db: db}
}

// CreateContract inserts a new contract.
func (r *ContractRepo) CreateContract(ctx context.Context, c *model.Contract) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO contracts
			(id, organization_id, uploaded_by, title, status, file_path, file_name,
			 file_size, file_mime_type, parties, tags, language, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,NOW(),NOW())`,
		c.ID, c.OrganizationID, c.UploadedBy, c.Title, c.Status,
		c.FilePath, c.FileName, c.FileSize, c.FileMimeType,
		c.Parties, c.Tags, c.Language,
	)
	if err != nil {
		return fmt.Errorf("contractRepo.CreateContract: %w", err)
	}
	return nil
}

// FindContractByID retrieves a contract by ID.
func (r *ContractRepo) FindContractByID(ctx context.Context, id string) (*model.Contract, error) {
	var c model.Contract
	err := r.db.GetContext(ctx, &c, `SELECT * FROM contracts WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("contractRepo.FindContractByID: %w", err)
	}
	return &c, nil
}

// ListContracts returns a paginated list of contracts for an org.
func (r *ContractRepo) ListContracts(ctx context.Context, orgID string, limit, offset int) ([]model.Contract, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM contracts WHERE organization_id = $1`, orgID); err != nil {
		return nil, 0, fmt.Errorf("contractRepo.ListContracts count: %w", err)
	}

	var contracts []model.Contract
	err := r.db.SelectContext(ctx, &contracts, `
		SELECT * FROM contracts
		WHERE organization_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`,
		orgID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("contractRepo.ListContracts: %w", err)
	}
	return contracts, total, nil
}

// CreateIngestionJob inserts a new ingestion job.
func (r *ContractRepo) CreateIngestionJob(ctx context.Context, job *model.IngestionJob) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ingestion_jobs (id, contract_id, status, progress, created_at, updated_at)
		VALUES ($1,$2,$3,$4,NOW(),NOW())`,
		job.ID, job.ContractID, job.Status, job.Progress)
	if err != nil {
		return fmt.Errorf("contractRepo.CreateIngestionJob: %w", err)
	}
	return nil
}

// FindIngestionJobByID retrieves an ingestion job by ID.
func (r *ContractRepo) FindIngestionJobByID(ctx context.Context, id string) (*model.IngestionJob, error) {
	var job model.IngestionJob
	err := r.db.GetContext(ctx, &job, `SELECT * FROM ingestion_jobs WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("contractRepo.FindIngestionJobByID: %w", err)
	}
	return &job, nil
}

// ListClausesByContractID returns all clauses for a contract ordered by index.
func (r *ContractRepo) ListClausesByContractID(ctx context.Context, contractID string) ([]model.Clause, error) {
	var clauses []model.Clause
	err := r.db.SelectContext(ctx, &clauses,
		`SELECT * FROM clauses WHERE contract_id = $1 ORDER BY clause_index`,
		contractID)
	if err != nil {
		return nil, fmt.Errorf("contractRepo.ListClausesByContractID: %w", err)
	}
	return clauses, nil
}

// DeleteContract removes a contract by ID scoped to the organization.
// Cascade deletes on clauses, ingestion_jobs, and risk_analyses are handled by the DB.
func (r *ContractRepo) DeleteContract(ctx context.Context, contractID, orgID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM contracts WHERE id = $1 AND organization_id = $2`,
		contractID, orgID)
	if err != nil {
		return fmt.Errorf("contractRepo.DeleteContract: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("contractRepo.DeleteContract rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("contractRepo.DeleteContract: not found or access denied")
	}
	return nil
}

// GetSnippet returns clauses that overlap with the given page and offset range.
func (r *ContractRepo) GetSnippet(ctx context.Context, contractID string, page, startOffset, endOffset int) ([]model.Clause, error) {
	var clauses []model.Clause
	err := r.db.SelectContext(ctx, &clauses, `
		SELECT * FROM clauses
		WHERE contract_id = $1
		  AND page_start <= $2 AND page_end >= $2
		  AND start_offset <= $4 AND end_offset >= $3
		ORDER BY clause_index`,
		contractID, page, startOffset, endOffset)
	if err != nil {
		return nil, fmt.Errorf("contractRepo.GetSnippet: %w", err)
	}
	return clauses, nil
}
