package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// EvidenceHandler handles evidence set HTTP requests.
type EvidenceHandler struct {
	evidenceSvc *service.EvidenceService
}

// NewEvidenceHandler creates a new EvidenceHandler.
func NewEvidenceHandler(evidenceSvc *service.EvidenceService) *EvidenceHandler {
	return &EvidenceHandler{evidenceSvc: evidenceSvc}
}

// GetEvidenceSet handles GET /evidence-sets/{evidenceSetId}
func (h *EvidenceHandler) GetEvidenceSet(w http.ResponseWriter, r *http.Request) {
	evidenceSetID := chi.URLParam(r, "evidenceSetId")

	es, err := h.evidenceSvc.GetEvidenceSet(r.Context(), evidenceSetID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to get evidence set")
		return
	}
	if es == nil {
		util.Error(w, http.StatusNotFound, "evidence set not found")
		return
	}
	util.JSON(w, http.StatusOK, es)
}

// RetrieveEvidence handles POST /evidence-sets/{evidenceSetId}/retrieve
func (h *EvidenceHandler) RetrieveEvidence(w http.ResponseWriter, r *http.Request) {
	evidenceSetID := chi.URLParam(r, "evidenceSetId")

	var req struct {
		TopK         int    `json:"topK"`
		FilterParams string `json:"filterParams"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TopK == 0 {
		req.TopK = 5
	}
	topKStr := strconv.Itoa(req.TopK)
	_ = topKStr

	if err := h.evidenceSvc.RetrieveEvidence(r.Context(), evidenceSetID, req.TopK, req.FilterParams); err != nil {
		util.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	util.JSON(w, http.StatusAccepted, map[string]string{
		"evidenceSetId": evidenceSetID,
		"status":        "retrieving",
	})
}
