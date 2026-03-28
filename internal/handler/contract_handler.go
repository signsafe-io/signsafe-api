package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

const maxUploadSize = 50 * 1024 * 1024 // 50 MB

// ContractHandler handles contract-related HTTP requests.
type ContractHandler struct {
	contractSvc *service.ContractService
}

// NewContractHandler creates a new ContractHandler.
func NewContractHandler(contractSvc *service.ContractService) *ContractHandler {
	return &ContractHandler{contractSvc: contractSvc}
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
		FileMimeType:   header.Header.Get("Content-Type"),
		File:           file,
	})
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "upload failed: "+err.Error())
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
	job, err := h.contractSvc.GetIngestionJob(r.Context(), jobID)
	if err != nil {
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

	w.WriteHeader(http.StatusNoContent)
}

// GetSnippets handles GET /contracts/{contractId}/snippets
func (h *ContractHandler) GetSnippets(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
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
