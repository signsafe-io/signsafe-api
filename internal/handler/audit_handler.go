package handler

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// AuditHandler handles audit event HTTP requests.
type AuditHandler struct {
	auditSvc *service.AuditService
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(auditSvc *service.AuditService) *AuditHandler {
	return &AuditHandler{auditSvc: auditSvc}
}

// CreateAuditEvent handles POST /audit-events
func (h *AuditHandler) CreateAuditEvent(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())

	var req struct {
		Action         string  `json:"action"`
		TargetType     *string `json:"targetType"`
		TargetID       *string `json:"targetId"`
		OrganizationID *string `json:"organizationId"`
		Context        string  `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Action == "" {
		util.Error(w, http.StatusBadRequest, "action is required")
		return
	}

	// Strip port from RemoteAddr so it can be stored in a PostgreSQL INET column.
	ipAddr := r.RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		ipAddr = host
	}
	userAgent := r.UserAgent()

	auditReq := service.CreateAuditEventRequest{
		Action:         req.Action,
		TargetType:     req.TargetType,
		TargetID:       req.TargetID,
		OrganizationID: req.OrganizationID,
		Context:        req.Context,
		IPAddress:      &ipAddr,
		UserAgent:      &userAgent,
	}

	if userID != "" {
		auditReq.ActorID = &userID
	}

	event, err := h.auditSvc.CreateAuditEvent(r.Context(), auditReq)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to create audit event")
		return
	}

	util.JSON(w, http.StatusCreated, event)
}
