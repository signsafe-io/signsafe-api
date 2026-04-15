package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// ContractUpdate holds the optional fields that can be patched on a contract.
// Only non-nil fields are written to the DB.
type ContractUpdate struct {
	Title        *string
	Tags         *string
	Parties      *string
	Language     *string
	ContractType *string
	SignedAt     *time.Time
	ExpiresAt    *time.Time
}

// ContractFilter holds optional search/filter parameters for ListContracts.
type ContractFilter struct {
	Q      string // title ILIKE search
	Status string // exact status match
}

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

// ListContracts returns a paginated list of contracts for an org with optional search/filter.
func (r *ContractRepo) ListContracts(ctx context.Context, orgID string, limit, offset int, filter ContractFilter) ([]model.Contract, int, error) {
	conditions := []string{"organization_id = $1"}
	args := []interface{}{orgID}
	argIdx := 2

	if filter.Q != "" {
		conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Q+"%")
		argIdx++
	}
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	if err := r.db.GetContext(ctx, &total,
		fmt.Sprintf(`SELECT COUNT(*) FROM contracts WHERE %s`, whereClause),
		args...); err != nil {
		return nil, 0, fmt.Errorf("contractRepo.ListContracts count: %w", err)
	}

	listArgs := append(args, limit, offset)
	var contracts []model.Contract
	// Subquery paginates the contracts, then LATERAL JOIN adds the latest
	// completed overall_risk per contract (null when no completed analysis exists).
	err := r.db.SelectContext(ctx, &contracts, fmt.Sprintf(`
		SELECT c.*,
		       ra.overall_risk AS latest_analysis_risk
		FROM (
		    SELECT * FROM contracts
		    WHERE %s
		    ORDER BY created_at DESC
		    LIMIT $%d OFFSET $%d
		) c
		LEFT JOIN LATERAL (
		    SELECT overall_risk
		    FROM risk_analyses
		    WHERE contract_id = c.id
		      AND status = 'completed'
		    ORDER BY created_at DESC
		    LIMIT 1
		) ra ON true
		ORDER BY c.created_at DESC`, whereClause, argIdx, argIdx+1),
		listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("contractRepo.ListContracts: %w", err)
	}
	return contracts, total, nil
}

// ListExpiringContracts returns contracts expiring within the given number of days,
// ordered by expires_at ascending (soonest first).
// Only contracts with a non-null expires_at in the future (and within the window) are returned.
func (r *ContractRepo) ListExpiringContracts(ctx context.Context, orgID string, days int) ([]model.Contract, error) {
	var contracts []model.Contract
	err := r.db.SelectContext(ctx, &contracts, `
		SELECT * FROM contracts
		WHERE organization_id = $1
		  AND expires_at IS NOT NULL
		  AND expires_at > NOW()
		  AND expires_at <= NOW() + ($2 * INTERVAL '1 day')
		ORDER BY expires_at ASC`,
		orgID, days)
	if err != nil {
		return nil, fmt.Errorf("contractRepo.ListExpiringContracts: %w", err)
	}
	return contracts, nil
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

// ListIngestionJobsByContractID returns all ingestion jobs for a contract, newest first.
func (r *ContractRepo) ListIngestionJobsByContractID(ctx context.Context, contractID string) ([]model.IngestionJob, error) {
	var jobs []model.IngestionJob
	err := r.db.SelectContext(ctx, &jobs,
		`SELECT * FROM ingestion_jobs WHERE contract_id = $1 ORDER BY created_at DESC`,
		contractID)
	if err != nil {
		return nil, fmt.Errorf("contractRepo.ListIngestionJobsByContractID: %w", err)
	}
	return jobs, nil
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

// UpdateContract applies a partial update to a contract.
// Only non-nil fields in updates are written; at least one field must be set.
func (r *ContractRepo) UpdateContract(ctx context.Context, contractID string, updates ContractUpdate) (*model.Contract, error) {
	setClauses := make([]string, 0, 8)
	args := make([]interface{}, 0, 9)
	argIdx := 1

	if updates.Title != nil {
		setClauses = append(setClauses, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *updates.Title)
		argIdx++
	}
	if updates.Tags != nil {
		setClauses = append(setClauses, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, *updates.Tags)
		argIdx++
	}
	if updates.Parties != nil {
		setClauses = append(setClauses, fmt.Sprintf("parties = $%d", argIdx))
		args = append(args, *updates.Parties)
		argIdx++
	}
	if updates.Language != nil {
		setClauses = append(setClauses, fmt.Sprintf("language = $%d", argIdx))
		args = append(args, *updates.Language)
		argIdx++
	}
	if updates.ContractType != nil {
		setClauses = append(setClauses, fmt.Sprintf("contract_type = $%d", argIdx))
		args = append(args, *updates.ContractType)
		argIdx++
	}
	if updates.SignedAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("signed_at = $%d", argIdx))
		args = append(args, *updates.SignedAt)
		argIdx++
	}
	if updates.ExpiresAt != nil {
		setClauses = append(setClauses, fmt.Sprintf("expires_at = $%d", argIdx))
		args = append(args, *updates.ExpiresAt)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil, fmt.Errorf("contractRepo.UpdateContract: no fields to update")
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, contractID)
	query := fmt.Sprintf(
		`UPDATE contracts SET %s WHERE id = $%d RETURNING *`,
		strings.Join(setClauses, ", "),
		argIdx,
	)

	var c model.Contract
	if err := r.db.GetContext(ctx, &c, query, args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("contractRepo.UpdateContract: not found")
		}
		return nil, fmt.Errorf("contractRepo.UpdateContract: %w", err)
	}
	return &c, nil
}

// GetSnippet returns clauses that overlap with the given page and offset range.
func (r *ContractRepo) GetSnippet(ctx context.Context, contractID string, page, startOffset, endOffset int) ([]model.Clause, error) {
	var clauses []model.Clause
	err := r.db.SelectContext(ctx, &clauses, `
		SELECT * FROM clauses
		WHERE contract_id = $1
		  AND page_start <= $2 AND page_end >= $2
		  AND start_offset <= $3 AND end_offset >= $4
		ORDER BY clause_index`,
		contractID, page, startOffset, endOffset)
	if err != nil {
		return nil, fmt.Errorf("contractRepo.GetSnippet: %w", err)
	}
	return clauses, nil
}
