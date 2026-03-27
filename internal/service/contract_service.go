package service

import (
	"context"
	"fmt"
	"io"

	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/queue"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/storage"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// ContractService handles contract business logic.
type ContractService struct {
	repo          *repository.ContractRepo
	queue         *queue.Client
	storageClient *storage.Client
}

// NewContractService creates a new ContractService.
func NewContractService(repo *repository.ContractRepo, q *queue.Client, s *storage.Client) *ContractService {
	return &ContractService{repo: repo, queue: q, storageClient: s}
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
