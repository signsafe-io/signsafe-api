package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

// AuthHandler handles auth-related HTTP requests.
type AuthHandler struct {
	authSvc *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Signup handles POST /auth/signup
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		FullName string `json:"fullName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		util.Error(w, http.StatusBadRequest, "email, password, and fullName are required")
		return
	}

	result, err := h.authSvc.Signup(r.Context(), service.SignupRequest{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
	})
	if err != nil {
		if strings.Contains(err.Error(), "email already registered") {
			util.Error(w, http.StatusConflict, "email already registered")
		} else {
			util.Error(w, http.StatusInternalServerError, "signup failed")
		}
		return
	}

	util.JSON(w, http.StatusCreated, map[string]interface{}{
		"userId":         result.UserID,
		"organizationId": result.OrganizationID,
		"message":        "verification email sent",
	})
}

// VerifyEmail handles POST /auth/verify-email
func (h *AuthHandler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" {
		util.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	if err := h.authSvc.VerifyEmail(r.Context(), req.Token); err != nil {
		util.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	util.JSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		util.Error(w, http.StatusBadRequest, "email and password are required")
		return
	}

	pair, err := h.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		msg := "invalid credentials"
		if strings.Contains(err.Error(), "email not verified") {
			msg = "email not verified"
		}
		util.Error(w, http.StatusUnauthorized, msg)
		return
	}

	// Set refresh token as httpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/auth",
		MaxAge:   30 * 24 * 60 * 60,
	})

	util.JSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": pair.AccessToken,
		"expiresAt":   pair.ExpiresAt,
	})
}

// Refresh handles POST /auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		util.Error(w, http.StatusUnauthorized, "refresh token not found")
		return
	}

	pair, err := h.authSvc.Refresh(r.Context(), cookie.Value)
	if err != nil {
		util.Error(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    pair.RefreshToken,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/auth",
		MaxAge:   30 * 24 * 60 * 60,
	})

	util.JSON(w, http.StatusOK, map[string]interface{}{
		"accessToken": pair.AccessToken,
		"expiresAt":   pair.ExpiresAt,
	})
}

// Logout handles POST /auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err == nil {
		_ = h.authSvc.Logout(r.Context(), cookie.Value)
	}

	// Clear the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		Path:     "/auth",
		MaxAge:   -1,
	})

	util.JSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// ForgotPassword handles POST /auth/password/forgot
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Always return 200 regardless of whether email exists (security).
	_ = h.authSvc.ForgotPassword(r.Context(), req.Email)
	util.JSON(w, http.StatusOK, map[string]string{"message": "if the email exists, a reset link has been sent"})
}

// ResetPassword handles POST /auth/password/reset
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		util.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.NewPassword == "" {
		util.Error(w, http.StatusBadRequest, "token and newPassword are required")
		return
	}

	if err := h.authSvc.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		util.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	util.JSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// GetMe handles GET /users/me
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		util.Error(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	result, err := h.authSvc.GetMe(r.Context(), userID)
	if err != nil {
		util.Error(w, http.StatusInternalServerError, "failed to fetch user")
		return
	}

	u := result.User
	util.JSON(w, http.StatusOK, map[string]interface{}{
		"id":             u.ID,
		"email":          u.Email,
		"fullName":       u.FullName,
		"role":           u.Role,
		"emailVerified":  u.EmailVerified,
		"mfaEnabled":     u.MFAEnabled,
		"createdAt":      u.CreatedAt,
		"permissions":    defaultPermissions(u.Role),
		"organizationId": result.OrganizationID,
	})
}

// defaultPermissions returns a permission slice based on the user role.
func defaultPermissions(role string) []string {
	switch role {
	case "admin":
		return []string{
			"contracts:read", "contracts:write",
			"analysis:read", "analysis:write",
			"evidence:read",
			"override:write",
			"audit:read",
			"users:read", "users:write",
		}
	case "reviewer":
		return []string{
			"contracts:read",
			"analysis:read",
			"evidence:read",
			"override:write",
			"audit:read",
		}
	default: // member
		return []string{
			"contracts:read", "contracts:write",
			"analysis:read", "analysis:write",
			"evidence:read",
		}
	}
}
