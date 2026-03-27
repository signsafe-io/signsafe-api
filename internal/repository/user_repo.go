package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
)

// UserRepo handles DB operations for users and tokens.
type UserRepo struct {
	db *sqlx.DB
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create inserts a new user.
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role,
		                   email_verified, email_verify_token, email_verify_expires_at,
		                   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Role,
		u.EmailVerified, u.EmailVerifyToken, u.EmailVerifyExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	return nil
}

// FindByEmail retrieves a user by email.
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE email = $1`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindByEmail: %w", err)
	}
	return &u, nil
}

// FindByID retrieves a user by ID.
func (r *UserRepo) FindByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE id = $1`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindByID: %w", err)
	}
	return &u, nil
}

// FindByEmailVerifyToken retrieves a user by email verify token.
func (r *UserRepo) FindByEmailVerifyToken(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE email_verify_token = $1 AND email_verify_expires_at > NOW()`,
		token)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindByEmailVerifyToken: %w", err)
	}
	return &u, nil
}

// FindByPasswordResetToken retrieves a user by password reset token.
func (r *UserRepo) FindByPasswordResetToken(ctx context.Context, token string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u,
		`SELECT * FROM users WHERE password_reset_token = $1 AND password_reset_expires_at > NOW()`,
		token)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindByPasswordResetToken: %w", err)
	}
	return &u, nil
}

// MarkEmailVerified sets email_verified=true and clears the token.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email_verified = TRUE,
		    email_verify_token = NULL,
		    email_verify_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("userRepo.MarkEmailVerified: %w", err)
	}
	return nil
}

// SetPasswordResetToken stores a password reset token.
func (r *UserRepo) SetPasswordResetToken(ctx context.Context, id, token string, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_reset_token = $1,
		    password_reset_expires_at = $2,
		    updated_at = NOW()
		WHERE id = $3`, token, expires, id)
	if err != nil {
		return fmt.Errorf("userRepo.SetPasswordResetToken: %w", err)
	}
	return nil
}

// UpdatePassword updates the password hash and clears the reset token.
func (r *UserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET password_hash = $1,
		    password_reset_token = NULL,
		    password_reset_expires_at = NULL,
		    updated_at = NOW()
		WHERE id = $2`, hash, id)
	if err != nil {
		return fmt.Errorf("userRepo.UpdatePassword: %w", err)
	}
	return nil
}

// CreateRefreshToken stores a new refresh token record.
func (r *UserRepo) CreateRefreshToken(ctx context.Context, rt *model.RefreshToken) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, revoked, created_at)
		VALUES ($1, $2, $3, $4, FALSE, NOW())`,
		rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt)
	if err != nil {
		return fmt.Errorf("userRepo.CreateRefreshToken: %w", err)
	}
	return nil
}

// FindRefreshToken retrieves a non-revoked, non-expired refresh token by hash.
func (r *UserRepo) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.db.GetContext(ctx, &rt, `
		SELECT * FROM refresh_tokens
		WHERE token_hash = $1 AND revoked = FALSE AND expires_at > NOW()`,
		tokenHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindRefreshToken: %w", err)
	}
	return &rt, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *UserRepo) RevokeRefreshToken(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("userRepo.RevokeRefreshToken: %w", err)
	}
	return nil
}

// RevokeAllRefreshTokens revokes all tokens for a user (logout from all devices).
func (r *UserRepo) RevokeAllRefreshTokens(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE refresh_tokens SET revoked = TRUE WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("userRepo.RevokeAllRefreshTokens: %w", err)
	}
	return nil
}
