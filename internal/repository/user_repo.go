package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/util"
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

// SetEmailVerifyToken replaces the current email verification token with a new one.
func (r *UserRepo) SetEmailVerifyToken(ctx context.Context, id, token string, expires time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET email_verify_token = $1,
		    email_verify_expires_at = $2,
		    updated_at = NOW()
		WHERE id = $3`, token, expires, id)
	if err != nil {
		return fmt.Errorf("userRepo.SetEmailVerifyToken: %w", err)
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

// FindRefreshTokenByHash retrieves any refresh token by hash (including revoked ones).
// Used to detect refresh token reuse attacks and identify the token owner.
func (r *UserRepo) FindRefreshTokenByHash(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	var rt model.RefreshToken
	err := r.db.GetContext(ctx, &rt, `
		SELECT * FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindRefreshTokenByHash: %w", err)
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

// IsOrgMember returns true when the user belongs to the given organization.
func (r *UserRepo) IsOrgMember(ctx context.Context, userID, orgID string) (bool, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `
		SELECT COUNT(*)
		FROM user_organizations
		WHERE user_id = $1 AND organization_id = $2`,
		userID, orgID)
	if err != nil {
		return false, fmt.Errorf("userRepo.IsOrgMember: %w", err)
	}
	return count > 0, nil
}

// GetMemberRole returns the role of a user in an org, or "" if not a member.
func (r *UserRepo) GetMemberRole(ctx context.Context, userID, orgID string) (string, error) {
	var role string
	err := r.db.GetContext(ctx, &role, `
		SELECT role
		FROM user_organizations
		WHERE user_id = $1 AND organization_id = $2`,
		userID, orgID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("userRepo.GetMemberRole: %w", err)
	}
	return role, nil
}

// UserOrganizationRow holds an organization together with the requesting user's role in it.
type UserOrganizationRow struct {
	ID   string `db:"id"`
	Name string `db:"name"`
	Plan string `db:"plan"`
	Role string `db:"role"`
}

// ListUserOrganizations returns all organizations a user belongs to, including the user's role in each.
func (r *UserRepo) ListUserOrganizations(ctx context.Context, userID string) ([]UserOrganizationRow, error) {
	var rows []UserOrganizationRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT o.id, o.name, o.plan, uo.role
		FROM organizations o
		JOIN user_organizations uo ON uo.organization_id = o.id
		WHERE uo.user_id = $1
		ORDER BY uo.joined_at ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListUserOrganizations: %w", err)
	}
	return rows, nil
}

// CreateOrganizationWithAdmin inserts a new organization and adds the given user as admin in a single transaction.
func (r *UserRepo) CreateOrganizationWithAdmin(ctx context.Context, org *model.Organization, userID string) error {
	memberID := util.NewID()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("userRepo.CreateOrganizationWithAdmin: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO organizations (id, name, plan, features, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`,
		org.ID, org.Name, org.Plan, org.Features,
	); err != nil {
		return fmt.Errorf("userRepo.CreateOrganizationWithAdmin: insert org: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_organizations (id, user_id, organization_id, role, permissions, joined_at)
		VALUES ($1, $2, $3, 'admin', '[]', NOW())`,
		memberID, userID, org.ID,
	); err != nil {
		return fmt.Errorf("userRepo.CreateOrganizationWithAdmin: insert membership: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("userRepo.CreateOrganizationWithAdmin: commit: %w", err)
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

// FindOrganizationByID returns an organization by ID.
func (r *UserRepo) FindOrganizationByID(ctx context.Context, orgID string) (*model.Organization, error) {
	var org model.Organization
	err := r.db.GetContext(ctx, &org, `SELECT * FROM organizations WHERE id = $1`, orgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindOrganizationByID: %w", err)
	}
	return &org, nil
}

// UpdateOrganizationName updates the name of an organization.
// Returns nil, nil if the organization does not exist.
func (r *UserRepo) UpdateOrganizationName(ctx context.Context, orgID, name string) (*model.Organization, error) {
	var org model.Organization
	err := r.db.GetContext(ctx, &org, `
		UPDATE organizations SET name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING *`, name, orgID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.UpdateOrganizationName: %w", err)
	}
	return &org, nil
}

// UpdateFullName updates a user's display name.
// Returns nil, nil if the user does not exist.
func (r *UserRepo) UpdateFullName(ctx context.Context, userID, fullName string) (*model.User, error) {
	var u model.User
	err := r.db.GetContext(ctx, &u, `
		UPDATE users SET full_name = $1, updated_at = NOW()
		WHERE id = $2
		RETURNING *`, fullName, userID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.UpdateFullName: %w", err)
	}
	return &u, nil
}

// OrgMember holds member info returned by ListOrgMembers.
type OrgMember struct {
	UserID        string    `db:"user_id"`
	Email         string    `db:"email"`
	FullName      string    `db:"full_name"`
	Role          string    `db:"role"`
	JoinedAt      time.Time `db:"joined_at"`
	EmailVerified bool      `db:"email_verified"`
}

// ListOrgMembers returns all members of an organization.
func (r *UserRepo) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	var members []OrgMember
	err := r.db.SelectContext(ctx, &members, `
		SELECT uo.user_id, u.email, u.full_name, uo.role, uo.joined_at, u.email_verified
		FROM user_organizations uo
		JOIN users u ON u.id = uo.user_id
		WHERE uo.organization_id = $1
		ORDER BY uo.joined_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListOrgMembers: %w", err)
	}
	return members, nil
}

// AddOrgMember adds a user to an organization with the given role.
func (r *UserRepo) AddOrgMember(ctx context.Context, userID, orgID, role string) error {
	id := util.NewID()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_organizations (id, user_id, organization_id, role, permissions, joined_at)
		VALUES ($1, $2, $3, $4, '[]', NOW())
		ON CONFLICT (user_id, organization_id) DO NOTHING`,
		id, userID, orgID, role)
	if err != nil {
		return fmt.Errorf("userRepo.AddOrgMember: %w", err)
	}
	return nil
}

// RemoveOrgMember removes a user from an organization.
func (r *UserRepo) RemoveOrgMember(ctx context.Context, userID, orgID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM user_organizations
		WHERE user_id = $1 AND organization_id = $2`, userID, orgID)
	if err != nil {
		return fmt.Errorf("userRepo.RemoveOrgMember: %w", err)
	}
	return nil
}

// UpdateMemberRole changes the role of a member in an organization.
func (r *UserRepo) UpdateMemberRole(ctx context.Context, userID, orgID, role string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE user_organizations SET role = $1
		WHERE user_id = $2 AND organization_id = $3`, role, userID, orgID)
	if err != nil {
		return fmt.Errorf("userRepo.UpdateMemberRole: %w", err)
	}
	return nil
}

// CreateInvitation stores a pending invitation.
func (r *UserRepo) CreateInvitation(ctx context.Context, inv *model.PendingInvitation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO pending_invitations
		    (id, organization_id, invited_by, email, role, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (organization_id, email)
		DO UPDATE SET token = EXCLUDED.token,
		              role = EXCLUDED.role,
		              expires_at = EXCLUDED.expires_at`,
		inv.ID, inv.OrganizationID, inv.InvitedBy, inv.Email, inv.Role, inv.Token, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("userRepo.CreateInvitation: %w", err)
	}
	return nil
}

// FindInvitationByToken retrieves a non-expired invitation by token.
func (r *UserRepo) FindInvitationByToken(ctx context.Context, token string) (*model.PendingInvitation, error) {
	var inv model.PendingInvitation
	err := r.db.GetContext(ctx, &inv, `
		SELECT * FROM pending_invitations
		WHERE token = $1 AND expires_at > NOW()`, token)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindInvitationByToken: %w", err)
	}
	return &inv, nil
}

// FindInvitationsByEmail retrieves all pending invitations for an email.
func (r *UserRepo) FindInvitationsByEmail(ctx context.Context, email string) ([]model.PendingInvitation, error) {
	var invs []model.PendingInvitation
	err := r.db.SelectContext(ctx, &invs, `
		SELECT * FROM pending_invitations
		WHERE email = $1 AND expires_at > NOW()`, email)
	if err != nil {
		return nil, fmt.Errorf("userRepo.FindInvitationsByEmail: %w", err)
	}
	return invs, nil
}

// DeleteInvitation removes a pending invitation by ID.
func (r *UserRepo) DeleteInvitation(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM pending_invitations WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("userRepo.DeleteInvitation: %w", err)
	}
	return nil
}

// OrgAdmin holds minimal info about an organization admin.
type OrgAdmin struct {
	UserID string `db:"user_id"`
	Email  string `db:"email"`
}

// ListAllOrganizations returns all organizations in the system.
// Used by background jobs that process every org.
func (r *UserRepo) ListAllOrganizations(ctx context.Context) ([]model.Organization, error) {
	var orgs []model.Organization
	err := r.db.SelectContext(ctx, &orgs, `SELECT * FROM organizations ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListAllOrganizations: %w", err)
	}
	return orgs, nil
}

// ListOrgAdmins returns all admin-role members of the given organization.
func (r *UserRepo) ListOrgAdmins(ctx context.Context, orgID string) ([]OrgAdmin, error) {
	var admins []OrgAdmin
	err := r.db.SelectContext(ctx, &admins, `
		SELECT uo.user_id, u.email
		FROM user_organizations uo
		JOIN users u ON u.id = uo.user_id
		WHERE uo.organization_id = $1
		  AND uo.role = 'admin'
		ORDER BY uo.joined_at ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("userRepo.ListOrgAdmins: %w", err)
	}
	return admins, nil
}
