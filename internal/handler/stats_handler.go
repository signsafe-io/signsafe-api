package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// StatsHandler handles dashboard statistics requests.
type StatsHandler struct {
	statsSvc *service.StatsService
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(statsSvc *service.StatsService) *StatsHandler {
	return &StatsHandler{statsSvc: statsSvc}
}

// GetOrgStats handles GET /organizations/{orgId}/stats
func (h *StatsHandler) GetOrgStats(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")

	stats, err := h.statsSvc.GetDashboardStats(r.Context(), userID, orgID)
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			util.Error(w, http.StatusForbidden, "access denied")
			return
		}
		util.Error(w, http.StatusInternalServerError, "failed to fetch stats")
		return
	}

	util.JSON(w, http.StatusOK, stats)
}
