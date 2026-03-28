package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/storage"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// ContractService handles contract business logic.
type ContractService struct {
	repo          *repository.ContractRepo
	userRepo      *repository.UserRepo
	queue         *queue.Client
	storageClient *storage.Client
}

// NewContractService creates a new ContractService.
func NewContractService(repo *repository.ContractRepo, userRepo *repository.UserRepo, q *queue.Client, s *storage.Client) *ContractService {
	return &ContractService{repo: repo, userRepo: userRepo, queue: q, storageClient: s}
}

// UploadRequest holds the contract upload parameters.
type UploadRequest struct {
	OrganizationID string
	UploadedBy     string
	Title          string
	FileName       string
	FileSize       int64
	FileMimeType   string
	File           io.Reader
}

// UploadResult is returned after a successful upload.
type UploadResult struct {
	ContractID string
	JobID      string
}

// Upload stores the file, creates DB records, and enqueues an ingestion job.
func (s *ContractService) Upload(ctx context.Context, req UploadRequest) (*UploadResult, error) {
	contractID := util.NewID()
	jobID := util.NewID()

	filePath := fmt.Sprintf("%s/%s/%s", req.OrganizationID, contractID, req.FileName)

	// 1. Store file in SeaweedFS.
	if err := s.storageClient.Put(ctx, filePath, req.File, req.FileMimeType); err != nil {
		return nil, fmt.Errorf("contractService.Upload: storage: %w", err)
	}

	// 2. Create contract record.
	c := &model.Contract{
		ID:             contractID,
		OrganizationID: req.OrganizationID,
		UploadedBy:     req.UploadedBy,
		Title:          req.Title,
		Status:         "uploaded",
		FilePath:       filePath,
		FileName:       req.FileName,
		FileSize:       req.FileSize,
		FileMimeType:   req.FileMimeType,
		Parties:        "[]",
		Tags:           "[]",
		Language:       "ko",
	}
	if err := s.repo.CreateContract(ctx, c); err != nil {
		return nil, fmt.Errorf("contractService.Upload: db: %w", err)
	}

	// 3. Create ingestion job.
	job := &model.IngestionJob{
		ID:         jobID,
		ContractID: contractID,
		Status:     "pending",
		Progress:   0,
	}
	if err := s.repo.CreateIngestionJob(ctx, job); err != nil {
		return nil, fmt.Errorf("contractService.Upload: create job: %w", err)
	}

	// 4. Enqueue ingestion message.
	msg := queue.IngestionMessage{
		ContractID: contractID,
		JobID:      jobID,
		FilePath:   filePath,
	}
	if err := s.queue.Publish(ctx, "ingestion.jobs", msg); err != nil {
		return nil, fmt.Errorf("contractService.Upload: queue: %w", err)
	}

	return &UploadResult{ContractID: contractID, JobID: jobID}, nil
}

// GetIngestionJob returns the current status of an ingestion job.
func (s *ContractService) GetIngestionJob(ctx context.Context, jobID string) (*model.IngestionJob, error) {
	job, err := s.repo.FindIngestionJobByID(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("contractService.GetIngestionJob: %w", err)
	}
	return job, nil
}

// IsOrgMember returns true when userID belongs to orgID.
func (s *ContractService) IsOrgMember(ctx context.Context, userID, orgID string) (bool, error) {
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return false, fmt.Errorf("contractService.IsOrgMember: %w", err)
	}
	return member, nil
}

// ListContracts returns a paginated list of contracts.
func (s *ContractService) ListContracts(ctx context.Context, orgID string, page, pageSize int) ([]model.Contract, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	contracts, total, err := s.repo.ListContracts(ctx, orgID, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("contractService.ListContracts: %w", err)
	}
	return contracts, total, nil
}

// GetContract returns a single contract by ID.
func (s *ContractService) GetContract(ctx context.Context, contractID string) (*model.Contract, error) {
	c, err := s.repo.FindContractByID(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("contractService.GetContract: %w", err)
	}
	return c, nil
}

// ListClauses returns all clauses for a contract.
func (s *ContractService) ListClauses(ctx context.Context, contractID string) ([]model.Clause, error) {
	clauses, err := s.repo.ListClausesByContractID(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("contractService.ListClauses: %w", err)
	}
	return clauses, nil
}

// GetSnippets returns clauses matching a page/offset range.
func (s *ContractService) GetSnippets(ctx context.Context, contractID string, page, startOffset, endOffset int) ([]model.Clause, error) {
	clauses, err := s.repo.GetSnippet(ctx, contractID, page, startOffset, endOffset)
	if err != nil {
		return nil, fmt.Errorf("contractService.GetSnippets: %w", err)
	}
	return clauses, nil
}

// UpdateContractRequest holds the optional fields for a partial contract update.
type UpdateContractRequest struct {
	Title        *string
	Tags         *string
	Parties      *string
	Language     *string
	ContractType *string
	SignedAt     *string
	ExpiresAt    *string
}

// UpdateContract applies a partial update to a contract.
// requestedBy must be a member of the contract's organization.
func (s *ContractService) UpdateContract(ctx context.Context, contractID, requestedBy string, req UpdateContractRequest) (*model.Contract, error) {
	c, err := s.repo.FindContractByID(ctx, contractID)
	if err != nil {
		return nil, fmt.Errorf("contractService.UpdateContract: %w", err)
	}
	if c == nil {
		return nil, fmt.Errorf("contractService.UpdateContract: contract not found")
	}

	member, err := s.userRepo.IsOrgMember(ctx, requestedBy, c.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("contractService.UpdateContract: check membership: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("contractService.UpdateContract: access denied")
	}

	updates := repository.ContractUpdate{
		Title:        req.Title,
		Tags:         req.Tags,
		Parties:      req.Parties,
		Language:     req.Language,
		ContractType: req.ContractType,
	}

	if req.SignedAt != nil {
		t, err := time.Parse(time.RFC3339, *req.SignedAt)
		if err != nil {
			return nil, fmt.Errorf("contractService.UpdateContract: invalid signedAt: %w", err)
		}
		updates.SignedAt = &t
	}

	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return nil, fmt.Errorf("contractService.UpdateContract: invalid expiresAt: %w", err)
		}
		updates.ExpiresAt = &t
	}

	updated, err := s.repo.UpdateContract(ctx, contractID, updates)
	if err != nil {
		return nil, fmt.Errorf("contractService.UpdateContract: db: %w", err)
	}

	return updated, nil
}

// DeleteContract deletes a contract and its associated storage file.
// requestedBy must be a member of the contract's organization.
func (s *ContractService) DeleteContract(ctx context.Context, contractID, requestedBy string) error {
	c, err := s.repo.FindContractByID(ctx, contractID)
	if err != nil {
		return fmt.Errorf("contractService.DeleteContract: %w", err)
	}
	if c == nil {
		return fmt.Errorf("contractService.DeleteContract: contract not found")
	}

	member, err := s.userRepo.IsOrgMember(ctx, requestedBy, c.OrganizationID)
	if err != nil {
		return fmt.Errorf("contractService.DeleteContract: check membership: %w", err)
	}
	if !member {
		return fmt.Errorf("contractService.DeleteContract: access denied")
	}

	// Delete from storage (best-effort; DB row is the authoritative record).
	// Storage failures are logged but must not block DB deletion.
	if err := s.storageClient.Delete(ctx, c.FilePath); err != nil {
		slog.Warn("contractService.DeleteContract: storage delete failed (best-effort)",
			"contractId", contractID, "filePath", c.FilePath, "error", err)
	}

	if err := s.repo.DeleteContract(ctx, contractID, c.OrganizationID); err != nil {
		return fmt.Errorf("contractService.DeleteContract: db: %w", err)
	}

	return nil
}

// GetFile retrieves the raw file bytes for a contract from SeaweedFS.
// Returns the ReadCloser (caller must close it), the mime type, and any error.
func (s *ContractService) GetFile(ctx context.Context, contractID string) (io.ReadCloser, string, error) {
	c, err := s.repo.FindContractByID(ctx, contractID)
	if err != nil {
		return nil, "", fmt.Errorf("contractService.GetFile: %w", err)
	}
	if c == nil {
		return nil, "", fmt.Errorf("contractService.GetFile: contract not found")
	}

	body, err := s.storageClient.Get(ctx, c.FilePath)
	if err != nil {
		return nil, "", fmt.Errorf("contractService.GetFile: storage: %w", err)
	}

	return body, c.FileMimeType, nil
}
