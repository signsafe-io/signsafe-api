package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/signsafe-io/signsafe-api/internal/cache"
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
	userRepo  *repository.UserRepo
	cache     *cache.Client
	jwtSecret string
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *repository.UserRepo, cache *cache.Client, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		cache:     cache,
		jwtSecret: jwtSecret,
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
	UserID string
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
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

	if err := s.userRepo.Create(ctx, u); err != nil {
		return nil, fmt.Errorf("authService.Signup: %w", err)
	}

	// TODO: send verification email (email service not yet implemented)
	return &SignupResult{UserID: u.ID}, nil
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
func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken string) (*TokenPair, error) {
	tokenHash := hashToken(rawRefreshToken)

	// Check Redis cache first.
	cacheKey := "refresh:" + tokenHash
	cached, err := s.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, fmt.Errorf("authService.Refresh: cache: %w", err)
	}
	if cached == "revoked" {
		return nil, fmt.Errorf("authService.Refresh: token revoked")
	}

	rt, err := s.userRepo.FindRefreshToken(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("authService.Refresh: %w", err)
	}
	if rt == nil {
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

	// TODO: send reset email
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

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
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

// GetMe returns the full user details.
func (s *AuthService) GetMe(ctx context.Context, userID string) (*model.User, error) {
	u, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("authService.GetMe: %w", err)
	}
	if u == nil {
		return nil, fmt.Errorf("authService.GetMe: user not found")
	}
	return u, nil
}

// --- internal helpers ---

func (s *AuthService) issueTokens(ctx context.Context, u *model.User) (*TokenPair, error) {
	expiresAt := time.Now().Add(accessTokenTTL)
	claims := jwt.MapClaims{
		"userId": u.ID,
		"role":   u.Role,
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
