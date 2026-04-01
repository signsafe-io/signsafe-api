package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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

// ListAuditEvents handles GET /audit-events
//
// Query parameters:
//
//	organizationId (required)
//	action         — filter by action type (optional)
//	from           — ISO 8601 start date inclusive (optional)
//	to             — ISO 8601 end date inclusive (optional)
//	page           — 1-based page number (default 1)
//	pageSize       — items per page (default 30, max 100)
func (h *AuditHandler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	orgID := q.Get("organizationId")
	if orgID == "" {
		util.Error(w, http.StatusBadRequest, "organizationId is required")
		return
	}

	// Verify the calling user belongs to the requested organization.
	userID := middleware.UserIDFromContext(r.Context())
	orgIDFromCtx := middleware.OrgIDFromContext(r.Context())
	if orgIDFromCtx != "" && orgIDFromCtx != orgID {
		util.Error(w, http.StatusForbidden, "access denied")
		return
	}
	_ = userID // used implicitly via auth middleware

	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("pageSize"))

	req := service.ListAuditEventsRequest{
		OrganizationID: orgID,
		Action:         q.Get("action"),
		Page:           page,
		PageSize:       pageSize,
	}

	if fromStr := q.Get("from"); fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			// Accept date-only format too.
			t, err = time.Parse("2006-01-02", fromStr)
			if err != nil {
				util.Error(w, http.StatusBadRequest, "invalid 'from' date (use ISO 8601)")
				return
			}
		}
		req.From = &t
	}
	if toStr := q.Get("to"); toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			t, err = time.Parse("2006-01-02", toStr)
			if err != nil {
				util.Error(w, http.StatusBadRequest, "invalid 'to' date (use ISO 8601)")
				return
			}
			// Inclusive end: shift to end of day.
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		req.To = &t
	}

	resp, err := h.auditSvc.ListAuditEvents(r.Context(), req)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to list audit events")
		return
	}

	util.JSON(w, http.StatusOK, resp)
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

	ipAddr := clientIP(r)
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
