package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/signsafe-io/signsafe-api/internal/cache"
	"github.com/signsafe-io/signsafe-api/internal/email"
	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/util"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 1 * time.Hour
	refreshTokenTTL = 30 * 24 * time.Hour
	resetTokenTTL   = 1 * time.Hour
	verifyTokenTTL  = 24 * time.Hour
)

// AuthService handles user authentication.
type AuthService struct {
	userRepo    *repository.UserRepo
	cache       *cache.Client
	emailClient *email.Client
	jwtSecret   string
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepo, cache *cache.Client, emailClient *email.Client, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		cache:       cache,
		emailClient: emailClient,
		jwtSecret:   jwtSecret,
	}
}

// SignupRequest holds signup fields.
type SignupRequest struct {
	Email    string
	Password string
	FullName string
}

// SignupResult holds the result after signup.
type SignupResult struct {
	UserID         string
	OrganizationID string
}

// Signup creates a new user account.
func (s *AuthService) Signup(ctx context.Context, req SignupRequest) (*SignupResult, error) {
	existing, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("authService.Signup: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("authService.Signup: email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("authService.Signup: hash password: %w", err)
	}

	verifyToken := generateOpaqueToken()
	expires := time.Now().Add(verifyTokenTTL)

	u := &model.User{
		ID:                   util.NewID(),
		Email:                req.Email,
		PasswordHash:         string(hash),
		FullName:             req.FullName,
		Role:                 "member",
		EmailVerified:        false,
		EmailVerifyToken:     &verifyToken,
		EmailVerifyExpiresAt: &expires,
	}

	org := &model.Organization{
		ID:       util.NewID(),
		Name:     req.FullName + "'s Organization",
		Plan:     "free",
		Features: "{}",
	}

	uo := &model.UserOrganization{
		ID:             util.NewID(),
		UserID:         u.ID,
		OrganizationID: org.ID,
		Role:           "admin",
		Permissions:    "[]",
	}

	if err := s.userRepo.CreateWithOrg(ctx, u, org, uo); err != nil {
		return nil, fmt.Errorf("authService.Signup: %w", err)
	}

	// Send verification email — graceful: log on error, do not fail signup.
	if err := s.emailClient.SendVerificationEmail(u.Email, verifyToken); err != nil {
		slog.Warn("authService.Signup: failed to send verification email",
			"userId", u.ID, "error", err)
	}

	return &SignupResult{UserID: u.ID, OrganizationID: org.ID}, nil
}

// VerifyEmail confirms a user's email address.
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	u, err := s.userRepo.FindByEmailVerifyToken(ctx, token)
	if err != nil {
		return fmt.Errorf("authService.VerifyEmail: %w", err)
	}
	if u == nil {
		return fmt.Errorf("authService.VerifyEmail: invalid or expired token")
	}

	if err := s.userRepo.MarkEmailVerified(ctx, u.ID); err != nil {
		return fmt.Errorf("authService.VerifyEmail: %w", err)
	}
	return nil
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Login authenticates a user and issues tokens.
func (s *AuthService) Login(ctx context.Context, email, password string) (*TokenPair, error) {
	u, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authService.Login: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("authService.Login: invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("authService.Login: invalid credentials")
	}

	if !u.EmailVerified {
		return nil, fmt.Errorf("authService.Login: email not verified")
	}

	return s.issueTokens(ctx, u)
}

// Refresh rotates a refresh token and issues a new access token.
// If a previously revoked token is presented, all sessions for the owning user
// are invalidated (refresh token reuse / theft detection).
func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	tokenHash := hashToken(rawRefreshToken)

	// Check Redis cache first.
	cacheKey := "refresh:" + tokenHash
	cached, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("authService.Refresh: cache: %w", err)
	}
	if cached == "revoked" {
		// Token was already used — possible theft. Revoke all sessions for this user.
		s.invalidateFamilyFromHash(ctx, tokenHash)
		return nil, fmt.Errorf("authService.Refresh: token already used — all sessions revoked")
	}

	rt, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("authService.Refresh: %w", err)
	}
	if rt == nil {
		// Token not found in DB at all — could be expired or already revoked.
		// Look it up without filters to detect reuse of previously valid tokens.
		s.invalidateFamilyFromHash(ctx, tokenHash)
		return nil, fmt.Errorf("authService.Refresh: invalid or expired token")
	}

	// Revoke old token.
	if err := s.userRepo.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("authService.Refresh: revoke old: %w", err)
	}
	_ = s.cache.Set(ctx, cacheKey, "revoked", refreshTokenTTL)

	u, err := s.userRepo.FindByID(ctx, rt.UserID)
	if err != nil {
		return nil, fmt.Errorf("authService.Refresh: find user: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("authService.Refresh: user not found")
	}

	return s.issueTokens(ctx, u)
}

// Logout revokes the given refresh token.
func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := hashToken(rawRefreshToken)

	rt, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return fmt.Errorf("authService.Logout: %w", err)
	}
	if rt != nil {
		if err := s.userRepo.RevokeRefreshToken(ctx, rt.ID); err != nil {
			return fmt.Errorf("authService.Logout: %w", err)
		}
		cacheKey := "refresh:" + tokenHash
		_ = s.cache.Set(ctx, cacheKey, "revoked", refreshTokenTTL)
	}
	return nil
}

// ForgotPassword stores a reset token (always returns success to prevent enumeration).
func (s *AuthService) ForgotPassword(ctx context.Context, email string) error {
	u, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("authService.ForgotPassword: %w", err)
	}
	if u == nil {
		// Silently succeed — do not reveal existence of account.
		return nil
	}

	resetToken := generateOpaqueToken()
	expires := time.Now().Add(resetTokenTTL)
	if err := s.userRepo.SetPasswordResetToken(ctx, u.ID, resetToken, expires); err != nil {
		return fmt.Errorf("authService.ForgotPassword: %w", err)
	}

	// Send reset email — graceful: log on error, do not expose user existence.
	if err := s.emailClient.SendPasswordResetEmail(u.Email, resetToken); err != nil {
		slog.Warn("authService.ForgotPassword: failed to send reset email",
			"userId", u.ID, "error", err)
	}

	return nil
}

// ResetPassword sets a new password using a reset token.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	u, err := s.userRepo.FindByPasswordResetToken(ctx, token)
	if err != nil {
		return fmt.Errorf("authService.ResetPassword: %w", err)
	}
	if u == nil {
		return fmt.Errorf("authService.ResetPassword: invalid or expired token")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("authService.ResetPassword: hash: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, u.ID, string(hash)); err != nil {
		return fmt.Errorf("authService.ResetPassword: %w", err)
	}

	// Revoke all existing refresh tokens for this user.
	if err := s.userRepo.RevokeAllRefreshTokens(ctx, u.ID); err != nil {
		return fmt.Errorf("authService.ResetPassword: revoke tokens: %w", err)
	}
	return nil
}

// ResendVerification issues a new email-verification token and sends it.
// Always returns nil to prevent account enumeration.
func (s *AuthService) ResendVerification(ctx context.Context, emailAddr string) error {
	u, err := s.userRepo.FindByEmail(ctx, emailAddr)
	if err != nil {
		return fmt.Errorf("authService.ResendVerification: %w", err)
	}
	if u == nil || u.EmailVerified {
		// Silently succeed — do not reveal whether the address exists or is already verified.
		return nil
	}

	verifyToken := generateOpaqueToken()
	expires := time.Now().Add(verifyTokenTTL)
	if err := s.userRepo.SetEmailVerifyToken(ctx, u.ID, verifyToken, expires); err != nil {
		return fmt.Errorf("authService.ResendVerification: %w", err)
	}

	if err := s.emailClient.SendVerificationEmail(u.Email, verifyToken); err != nil {
		slog.Warn("authService.ResendVerification: failed to send email",
			"userId", u.ID, "error", err)
	}
	return nil
}

// GetMeResult holds user details together with the primary organization.
type GetMeResult struct {
	User             *model.User
	OrganizationID   string
	OrganizationName string
}

// GetMe returns the full user details with their primary organization ID and name.
func (s *AuthService) GetMe(ctx context.Context, userID string) (*GetMeResult, error) {
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("authService.GetMe: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("authService.GetMe: user not found")
	}

	org, err := s.userRepo.FindOrganizationByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("authService.GetMe: find org: %w", err)
	}

	orgID, orgName := "", ""
	if org != nil {
		orgID = org.ID
		orgName = org.Name
	}

	return &GetMeResult{User: u, OrganizationID: orgID, OrganizationName: orgName}, nil
}

// --- internal helpers ---

func (s *AuthService) issueTokens(ctx context.Context, u *model.User) (*TokenPair, error) {
	expiresAt := time.Now().Add(accessTokenTTL)

	// Embed orgId so handlers can avoid a DB round-trip for the common case.
	// The IDOR checks in each handler still verify membership explicitly.
	var orgID string
	org, err := s.userRepo.FindOrganizationByUserID(ctx, u.ID)
	if err != nil {
		slog.Warn("issueTokens: could not look up org for user", "userId", u.ID, "err", err)
	} else if org != nil {
		orgID = org.ID
	}

	claims := jwt.MapClaims{
		"userId": u.ID,
		"role":   u.Role,
		"orgId":  orgID,
		"exp":    expiresAt.Unix(),
		"iat":    time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("issueTokens: sign: %w", err)
	}

	rawRefresh := generateOpaqueToken()
	tokenHash := hashToken(rawRefresh)
	rt := &model.RefreshToken{
		ID:        util.NewID(),
		UserID:    u.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}
	if err := s.userRepo.CreateRefreshToken(ctx, rt); err != nil {
		return nil, fmt.Errorf("issueTokens: store refresh token: %w", err)
	}

	// Cache the token hash so validation is fast.
	cacheKey := "refresh:" + tokenHash
	_ = s.cache.Set(ctx, cacheKey, u.ID, refreshTokenTTL)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    expiresAt,
	}, nil
}

// invalidateFamilyFromHash looks up the user who owns the given token hash
// (including revoked tokens) and revokes all their refresh tokens.
// This is a best-effort operation; errors are logged but not propagated.
func (s *AuthService) invalidateFamilyFromHash(ctx context.Context, tokenHash string) {
	rt, err := s.userRepo.FindRefreshTokenByHash(ctx, tokenHash)
	if err != nil || rt == nil {
		// Token not in DB at all — nothing we can do.
		slog.Warn("authService: token reuse detected but owner not found", "tokenHash", tokenHash[:8])
		return
	}
	if err := s.userRepo.RevokeAllRefreshTokens(ctx, rt.UserID); err != nil {
		slog.Warn("authService: failed to revoke all tokens on reuse detection",
			"userId", rt.UserID, "error", err)
		return
	}
	slog.Warn("authService: refresh token reuse detected — all sessions revoked",
		"userId", rt.UserID)
}

// generateOpaqueToken returns a cryptographically random hex token (32 bytes = 64 chars).
func generateOpaqueToken() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic(fmt.Errorf("generateOpaqueToken: %w", err))
	}
	return hex.EncodeToString(b)
}

// hashToken returns the SHA-256 hex digest of the raw token.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// UpdateProfile updates the display name of the authenticated user.
func (s *AuthService) UpdateProfile(ctx context.Context, userID, fullName string) (*model.User, error) {
	if fullName == "" {
		return nil, fmt.Errorf("authService.UpdateProfile: fullName cannot be empty")
	}
	u, err := s.userRepo.UpdateFullName(ctx, userID, fullName)
	if err != nil {
		return nil, fmt.Errorf("authService.UpdateProfile: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("authService.UpdateProfile: user not found")
	}
	return u, nil
}

// ChangePassword verifies the current password and sets a new one.
func (s *AuthService) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	if len(newPassword) < 8 {
		return fmt.Errorf("authService.ChangePassword: new password must be at least 8 characters")
	}
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("authService.ChangePassword: %w", err)
	}
	if u == nil {
		return fmt.Errorf("authService.ChangePassword: user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("authService.ChangePassword: current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("authService.ChangePassword: hash: %w", err)
	}
	if err := s.userRepo.UpdatePassword(ctx, userID, string(hash)); err != nil {
		return fmt.Errorf("authService.ChangePassword: %w", err)
	}
	// Revoke all existing sessions so stolen tokens can't be reused.
	if err := s.userRepo.RevokeAllRefreshTokens(ctx, userID); err != nil {
		slog.Warn("authService.ChangePassword: failed to revoke refresh tokens", "userId", userID, "error", err)
	}
	return nil
}
