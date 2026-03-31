package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// OrgHandler handles organization and user-profile HTTP requests.
type OrgHandler struct {
	orgSvc  *service.OrgService
	authSvc *service.AuthService
}

// NewOrgHandler creates a new OrgHandler.
func NewOrgHandler(orgSvc *service.OrgService, authSvc *service.AuthService) *OrgHandler {
	return &OrgHandler{orgSvc: orgSvc, authSvc: authSvc}
}

// ListMyOrganizations handles GET /users/me/organizations
func (h *OrgHandler) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgs, err := h.orgSvc.ListMyOrganizations(r.Context(), userID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to list organizations")
		return
	}
	util.JSON(w, http.StatusOK, orgs)
}

// CreateOrganization handles POST /organizations
func (h *OrgHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == "" {
		util.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	org, err := h.orgSvc.CreateOrganization(r.Context(), userID, body.Name)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to create organization")
		return
	}
	util.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":   org.ID,
		"name": org.Name,
		"plan": org.Plan,
	})
}

// UpdateProfile handles PATCH /users/me
func (h *OrgHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var body struct {
		FullName string `json:"fullName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.FullName == "" {
		util.Error(w, http.StatusBadRequest, "fullName is required")
		return
	}
	u, err := h.authSvc.UpdateProfile(r.Context(), userID, body.FullName)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	util.JSON(w, http.StatusOK, map[string]interface{}{
		"id":       u.ID,
		"email":    u.Email,
		"fullName": u.FullName,
	})
}

// ChangePassword handles PATCH /users/me/password
func (h *OrgHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.CurrentPassword == "" || body.NewPassword == "" {
		util.Error(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}
	if err := h.authSvc.ChangePassword(r.Context(), userID, body.CurrentPassword, body.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrWrongPassword):
			util.Error(w, http.StatusUnauthorized, "current password is incorrect")
		case errors.Is(err, service.ErrPasswordTooShort):
			util.Error(w, http.StatusBadRequest, "new password must be at least 8 characters")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to change password")
		}
		return
	}
	util.JSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// GetOrganization handles GET /organizations/{orgId}
func (h *OrgHandler) GetOrganization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	org, err := h.orgSvc.GetOrganization(r.Context(), userID, orgID)
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			util.Error(w, http.StatusForbidden, "access denied")
			return
		}
		util.Error(w, http.StatusInternalServerError, "failed to get organization")
		return
	}
	if org == nil {
		util.Error(w, http.StatusNotFound, "organization not found")
		return
	}
	util.JSON(w, http.StatusOK, org)
}

// UpdateOrganization handles PATCH /organizations/{orgId}
func (h *OrgHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Name == "" {
		util.Error(w, http.StatusBadRequest, "name is required")
		return
	}
	org, err := h.orgSvc.UpdateOrganization(r.Context(), userID, orgID, body.Name)
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			util.Error(w, http.StatusForbidden, "access denied")
			return
		}
		util.Error(w, http.StatusInternalServerError, "failed to update organization")
		return
	}
	util.JSON(w, http.StatusOK, org)
}

// ListMembers handles GET /organizations/{orgId}/members
func (h *OrgHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	members, err := h.orgSvc.ListMembers(r.Context(), userID, orgID)
	if err != nil {
		if errors.Is(err, service.ErrAccessDenied) {
			util.Error(w, http.StatusForbidden, "access denied")
			return
		}
		util.Error(w, http.StatusInternalServerError, "failed to list members")
		return
	}
	util.JSON(w, http.StatusOK, map[string]interface{}{
		"members": members,
		"total":   len(members),
	})
}

// InviteMember handles POST /organizations/{orgId}/members
func (h *OrgHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Email == "" {
		util.Error(w, http.StatusBadRequest, "email is required")
		return
	}
	if err := h.orgSvc.InviteMember(r.Context(), userID, orgID, body.Email, body.Role); err != nil {
		switch {
		case errors.Is(err, service.ErrAccessDenied):
			util.Error(w, http.StatusForbidden, "access denied")
		case errors.Is(err, service.ErrInvalidInput):
			util.Error(w, http.StatusBadRequest, "invalid role")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to invite member")
		}
		return
	}
	util.JSON(w, http.StatusOK, map[string]string{"message": "member added"})
}

// UpdateMemberRole handles PATCH /organizations/{orgId}/members/{userId}
func (h *OrgHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	targetUserID := chi.URLParam(r, "userId")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if body.Role == "" {
		util.Error(w, http.StatusBadRequest, "role is required")
		return
	}
	if err := h.orgSvc.UpdateMemberRole(r.Context(), userID, orgID, targetUserID, body.Role); err != nil {
		switch {
		case errors.Is(err, service.ErrAccessDenied):
			util.Error(w, http.StatusForbidden, "access denied")
		case errors.Is(err, service.ErrInvalidInput):
			util.Error(w, http.StatusBadRequest, "invalid role")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to update member role")
		}
		return
	}
	util.JSON(w, http.StatusOK, map[string]string{"message": "role updated"})
}

// RemoveMember handles DELETE /organizations/{orgId}/members/{userId}
func (h *OrgHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	orgID := chi.URLParam(r, "orgId")
	targetUserID := chi.URLParam(r, "userId")
	if err := h.orgSvc.RemoveMember(r.Context(), userID, orgID, targetUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrAccessDenied):
			util.Error(w, http.StatusForbidden, "access denied")
		case errors.Is(err, service.ErrInvalidInput):
			util.Error(w, http.StatusBadRequest, "cannot remove yourself from organization")
		default:
			util.Error(w, http.StatusInternalServerError, "failed to remove member")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
