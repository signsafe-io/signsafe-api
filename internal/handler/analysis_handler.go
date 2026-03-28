package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// AnalysisHandler handles risk analysis HTTP requests.
type AnalysisHandler struct {
	analysisSvc *service.AnalysisService
}

// NewAnalysisHandler creates a new AnalysisHandler.
func NewAnalysisHandler(analysisSvc *service.AnalysisService) *AnalysisHandler {
	return &AnalysisHandler{analysisSvc: analysisSvc}
}

// GetLatestAnalysis handles GET /contracts/{contractId}/risk-analyses
func (h *AnalysisHandler) GetLatestAnalysis(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
	userID := middleware.UserIDFromContext(r.Context())

	analysis, results, err := h.analysisSvc.GetLatestAnalysis(r.Context(), contractID, userID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "contract not found"):
			util.Error(w, http.StatusNotFound, "contract not found")
		case strings.Contains(err.Error(), "access denied"):
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to get analysis")
		}
		return
	}
	if analysis == nil {
		util.Error(w, http.StatusNotFound, "no analysis found for this contract")
		return
	}

	util.JSON(w, http.StatusOK, map[string]interface{}{
		"analysis":      analysis,
		"clauseResults": results,
	})
}

// CreateAnalysis handles POST /contracts/{contractId}/risk-analyses
func (h *AnalysisHandler) CreateAnalysis(w http.ResponseWriter, r *http.Request) {
	contractID := chi.URLParam(r, "contractId")
	userID := middleware.UserIDFromContext(r.Context())

	analysisID, err := h.analysisSvc.CreateAnalysis(r.Context(), contractID, userID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "analysis already running"):
			util.Error(w, http.StatusConflict, "analysis already running for this contract")
		case strings.Contains(err.Error(), "contract not found"):
			util.Error(w, http.StatusNotFound, "contract not found")
		case strings.Contains(err.Error(), "access denied"):
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to create analysis")
		}
		return
	}

	util.JSON(w, http.StatusAccepted, map[string]string{
		"analysisId": analysisID,
		"status":     "pending",
	})
}

// GetAnalysis handles GET /risk-analyses/{analysisId}
func (h *AnalysisHandler) GetAnalysis(w http.ResponseWriter, r *http.Request) {
	analysisID := chi.URLParam(r, "analysisId")
	userID := middleware.UserIDFromContext(r.Context())

	analysis, results, err := h.analysisSvc.GetAnalysis(r.Context(), analysisID, userID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "access denied"):
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to get analysis")
		}
		return
	}
	if analysis == nil {
		util.Error(w, http.StatusNotFound, "analysis not found")
		return
	}

	util.JSON(w, http.StatusOK, map[string]interface{}{
		"analysis":      analysis,
		"clauseResults": results,
	})
}

// CreateOverride handles POST /risk-analyses/{analysisId}/overrides
func (h *AnalysisHandler) CreateOverride(w http.ResponseWriter, r *http.Request) {
	analysisID := chi.URLParam(r, "analysisId")
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		ClauseResultID string `json:"clauseResultId"`
		NewRiskLevel   string `json:"newRiskLevel"`
		Reason         string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ClauseResultID == "" || req.NewRiskLevel == "" || req.Reason == "" {
		util.Error(w, http.StatusBadRequest, "clauseResultId, newRiskLevel, and reason are required")
		return
	}

	override, err := h.analysisSvc.CreateOverride(r.Context(), analysisID, req.ClauseResultID, req.NewRiskLevel, req.Reason, userID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "invalid risk level"):
			util.Error(w, http.StatusBadRequest, "invalid risk level: must be HIGH, MEDIUM, or LOW")
		case strings.Contains(err.Error(), "analysis not found"):
			util.Error(w, http.StatusNotFound, "analysis not found")
		case strings.Contains(err.Error(), "clause result not found"):
			util.Error(w, http.StatusNotFound, "clause result not found")
		case strings.Contains(err.Error(), "does not belong to analysis"):
			util.Error(w, http.StatusForbidden, "clause result does not belong to this analysis")
		case strings.Contains(err.Error(), "access denied"):
			util.Error(w, http.StatusForbidden, "access denied: not a member of this organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to create override")
		}
		return
	}

	util.JSON(w, http.StatusCreated, override)
}
