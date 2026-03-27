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

// CreateOrganization inserts a new organization.
func (r *UserRepo) CreateOrganization(ctx context.Context, tx sqlx.ExtContext, org *model.Organization) error {
	_, err := sqlx.NamedExecContext(ctx, tx, `
		INSERT INTO organizations (id, name, plan, features, created_at, updated_at)
		VALUES (:id, :name, :plan, :features, NOW(), NOW())`, org)
	if err != nil {
		return fmt.Errorf("userRepo.CreateOrganization: %w", err)
	}
	return nil
}

// CreateUserOrganization inserts a user-organization membership.
func (r *UserRepo) CreateUserOrganization(ctx context.Context, tx sqlx.ExtContext, uo *model.UserOrganization) error {
	_, err := sqlx.NamedExecContext(ctx, tx, `
		INSERT INTO user_organizations (id, user_id, organization_id, role, permissions, joined_at)
		VALUES (:id, :user_id, :organization_id, :role, :permissions, NOW())`, uo)
	if err != nil {
		return fmt.Errorf("userRepo.CreateUserOrganization: %w", err)
	}
	return nil
}

// CreateWithOrg creates a user, their personal organization, and membership in a single transaction.
func (r *UserRepo) CreateWithOrg(ctx context.Context, u *model.User, org *model.Organization, uo *model.UserOrganization) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("userRepo.CreateWithOrg: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, full_name, role,
		                   email_verified, email_verify_token, email_verify_expires_at,
		                   created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())`,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Role,
		u.EmailVerified, u.EmailVerifyToken, u.EmailVerifyExpiresAt,
	); err != nil {
		return fmt.Errorf("userRepo.CreateWithOrg: insert user: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO organizations (id, name, plan, features, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		org.ID, org.Name, org.Plan, org.Features,
	); err != nil {
		return fmt.Errorf("userRepo.CreateWithOrg: insert org: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_organizations (id, user_id, organization_id, role, permissions, joined_at)
		VALUES ($1, $2, $3, $4, $5, NOW())`,
		uo.ID, uo.UserID, uo.OrganizationID, uo.Role, uo.Permissions,
	); err != nil {
		return fmt.Errorf("userRepo.CreateWithOrg: insert user_org: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("userRepo.CreateWithOrg: commit: %w", err)
	}
	return nil
}

// FindOrganizationByUserID returns the first organization a user belongs to.
func (r *UserRepo) FindOrganizationByUserID(ctx context.Context, userID string) (*model.Organization, error) {
	var org model.Organization
	err := r.db.GetContext(ctx, &org, `
		SELECT o.*
		FROM organizations o
		JOIN user_organizations uo ON uo.organization_id = o.id
		WHERE uo.user_id = $1
		ORDER BY uo.joined_at ASC
		LIMIT 1`, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindOrganizationByUserID: %w", err)
	}
	return &org, nil
}
