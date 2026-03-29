package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

const maxUploadSize = 50 * 1024 * 1024 // 50 MB

// allowedMimeTypes is the set of file types accepted for contract upload.
// Validated against actual magic bytes, not the client-supplied Content-Type.
var allowedMimeTypes = map[string]struct{}{
	"application/pdf": {},
}

// ContractHandler handles contract-related HTTP requests.
type ContractHandler struct {
	contractSvc *service.ContractService
	auditSvc    *service.AuditService
}

// NewContractHandler creates a new ContractHandler.
func NewContractHandler(contractSvc *service.ContractService, auditSvc *service.AuditService) *ContractHandler {
	return &ContractHandler{contractSvc: contractSvc, auditSvc: auditSvc}
}

// logAudit records an audit event in a best-effort, non-blocking way.
// A failure to write the audit log never blocks or fails the original request.
func (h *ContractHandler) logAudit(r *http.Request, action string, targetType, targetID, orgID *string) {
	ipAddr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ipAddr = host
	}
	userAgent := r.UserAgent()
	userID := middleware.UserIDFromContext(r.Context())

	req := service.CreateAuditEventRequest{
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		OrganizationID: orgID,
		IPAddress:      &ipAddr,
		UserAgent:      &userAgent,
	}
	if userID != "" {
		req.ActorID = &userID
	}
	// Fire-and-forget: use context.Background() so the audit write is not
	// cancelled when the HTTP request context is done.
	go func() {
		_, _ = h.auditSvc.CreateAuditEvent(context.Background(), req)
	}()
}

// Upload handles POST /contracts
func (h *ContractHandler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		util.Error(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		util.Error(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// Read entire file into memory so we can (a) detect MIME type and
	// (b) pass a seekable bytes.Reader to the S3 SDK (PutObject requires seek).
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		util.Error(w, http.StatusBadRequest, "failed to read file")
		return
	}
	detectedType := http.DetectContentType(fileBytes)
	if _, ok := allowedMimeTypes[detectedType]; !ok {
		util.Error(w, http.StatusUnsupportedMediaType,
			"unsupported file type: only PDF files are accepted")
		return
	}
	fullFile := bytes.NewReader(fileBytes)

	title := r.FormValue("title")
	if title == "" {
		title = header.Filename
	}
	orgID := r.FormValue("organizationId")
	if orgID == "" {
		util.Error(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	result, err := h.contractSvc.Upload(r.Context(), service.UploadRequest{
		OrganizationID: orgID,
		UploadedBy:     userID,
		Title:          title,
		FileName:       header.Filename,
		FileSize:       header.Size,
		FileMimeType:   detectedType,
		File:           fullFile,
	})
	if err != nil {
		slog.Error("contract upload failed", "error", err, "orgId", orgID, "userId", userID)
		util.Error(w, http.StatusInternalServerError, "upload failed")
		return
	}

	util.JSON(w, http.StatusCreated, map[string]string{
		"contractId": result.ContractID,
		"jobId":      result.JobID,
	})
}

// List handles GET /contracts
func (h *ContractHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organizationId")
	if orgID == "" {
		util.Error(w, http.StatusBadRequest, "organizationId query parameter is required")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.contractSvc.IsOrgMember(r.Context(), userID, orgID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to verify organization membership")
		return
	}
	if !member {
		util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	contracts, total, err := h.contractSvc.ListContracts(r.Context(), orgID, page, pageSize)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to list contracts")
		return
	}

	util.JSON(w, http.StatusOK, map[string]interface{}{
		"contracts": contracts,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
	})
}

// Get handles GET /contracts/{contractId}
func (h *ContractHandler) Get(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
	c, err := h.contractSvc.GetContract(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get contract")
		return
	}
	if c == nil {
		util.Error(w, http.StatusNotFound, "contract not found")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.contractSvc.IsOrgMember(r.Context(), userID, c.OrganizationID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to verify organization membership")
		return
	}
	if !member {
		util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		return
	}

	util.JSON(w, http.StatusOK, c)
}

// GetIngestionJob handles GET /ingestion-jobs/{jobId}
func (h *ContractHandler) GetIngestionJob(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "jobId")
	userID := middleware.UserIDFromContext(r.Context())

	job, err := h.contractSvc.GetIngestionJob(r.Context(), jobID, userID)
	if err != nil {
		if err.Error() == "contractService.GetIngestionJob: access denied" {
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
			return
		}
		util.Error(w, http.StatusInternalServerError, "failed to get ingestion job")
		return
	}
	if job == nil {
		util.Error(w, http.StatusNotFound, "ingestion job not found")
		return
	}
	util.JSON(w, http.StatusOK, job)
}

// ListClauses handles GET /contracts/{contractId}/clauses
func (h *ContractHandler) ListClauses(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")

	c, err := h.contractSvc.GetContract(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get contract")
		return
	}
	if c == nil {
		util.Error(w, http.StatusNotFound, "contract not found")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.contractSvc.IsOrgMember(r.Context(), userID, c.OrganizationID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to verify organization membership")
		return
	}
	if !member {
		util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		return
	}

	clauses, err := h.contractSvc.ListClauses(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to list clauses")
		return
	}
	util.JSON(w, http.StatusOK, map[string]interface{}{
		"clauses": clauses,
		"total":   len(clauses),
	})
}

// GetFile handles GET /contracts/{contractId}/file
func (h *ContractHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")

	c, err := h.contractSvc.GetContract(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get contract")
		return
	}
	if c == nil {
		util.Error(w, http.StatusNotFound, "contract not found")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.contractSvc.IsOrgMember(r.Context(), userID, c.OrganizationID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to verify organization membership")
		return
	}
	if !member {
		util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		return
	}

	body, contentType, err := h.contractSvc.GetFile(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to retrieve file")
		return
	}
	defer body.Close()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, c.FileName))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}

// Delete handles DELETE /contracts/{contractId}
func (h *ContractHandler) Delete(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
	userID := middleware.UserIDFromContext(r.Context())

	if err := h.contractSvc.DeleteContract(r.Context(), contractID, userID); err != nil {
		switch {
		case err.Error() == "contractService.DeleteContract: contract not found":
			util.Error(w, http.StatusNotFound, "contract not found")
		case err.Error() == "contractService.DeleteContract: access denied":
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to delete contract: "+err.Error())
		}
		return
	}

	targetType := "contract"
	h.logAudit(r, "CONTRACT_DELETED", &targetType, &contractID, nil)

	w.WriteHeader(http.StatusNoContent)
}

// updateContractBody is the JSON body accepted by PATCH /contracts/{contractId}.
type updateContractBody struct {
	Title        *string `json:"title"`
	Tags         *string `json:"tags"`
	Parties      *string `json:"parties"`
	Language     *string `json:"language"`
	ContractType *string `json:"contractType"`
	SignedAt     *string `json:"signedAt"`
	ExpiresAt    *string `json:"expiresAt"`
}

// Update handles PATCH /contracts/{contractId}
func (h *ContractHandler) Update(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
	userID := middleware.UserIDFromContext(r.Context())

	var body updateContractBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	req := service.UpdateContractRequest{
		Title:        body.Title,
		Tags:         body.Tags,
		Parties:      body.Parties,
		Language:     body.Language,
		ContractType: body.ContractType,
		SignedAt:     body.SignedAt,
		ExpiresAt:    body.ExpiresAt,
	}

	updated, err := h.contractSvc.UpdateContract(r.Context(), contractID, userID, req)
	if err != nil {
		switch {
		case err.Error() == "contractService.UpdateContract: contract not found":
			util.Error(w, http.StatusNotFound, "contract not found")
		case err.Error() == "contractService.UpdateContract: access denied":
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to update contract: "+err.Error())
		}
		return
	}

	targetType := "contract"
	orgID := updated.OrganizationID
	h.logAudit(r, "CONTRACT_UPDATED", &targetType, &contractID, &orgID)

	util.JSON(w, http.StatusOK, updated)
}

// GetSnippets handles GET /contracts/{contractId}/snippets
func (h *ContractHandler) GetSnippets(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")

	c, err := h.contractSvc.GetContract(r.Context(), contractID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get contract")
		return
	}
	if c == nil {
		util.Error(w, http.StatusNotFound, "contract not found")
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	member, err := h.contractSvc.IsOrgMember(r.Context(), userID, c.OrganizationID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to verify organization membership")
		return
	}
	if !member {
		util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	startOffset, _ := strconv.Atoi(r.URL.Query().Get("startOffset"))
	endOffset, _ := strconv.Atoi(r.URL.Query().Get("endOffset"))

	if page < 1 {
		page = 1
	}

	clauses, err := h.contractSvc.GetSnippets(r.Context(), contractID, page, startOffset, endOffset)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get snippets")
		return
	}
	util.JSON(w, http.StatusOK, map[string]interface{}{
		"snippets": clauses,
	})
}
