package model

import "time"

type User struct {
	ID                     string     `db:"id" json:"id"`
	Email                  string     `db:"email" json:"email"`
	PasswordHash           string     `db:"password_hash" json:"-"`
	FullName               string     `db:"full_name" json:"fullName"`
	Role                   string     `db:"role" json:"role"`
	EmailVerified          bool       `db:"email_verified" json:"emailVerified"`
	EmailVerifyToken       *string    `db:"email_verify_token" json:"-"`
	EmailVerifyExpiresAt   *time.Time `db:"email_verify_expires_at" json:"-"`
	PasswordResetToken     *string    `db:"password_reset_token" json:"-"`
	PasswordResetExpiresAt *time.Time `db:"password_reset_expires_at" json:"-"`
	MFAEnabled             bool       `db:"mfa_enabled" json:"mfaEnabled"`
	MFASecret              *string    `db:"mfa_secret" json:"-"`
	CreatedAt              time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt              time.Time  `db:"updated_at" json:"updatedAt"`
}

type Organization struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Plan      string    `db:"plan" json:"plan"`
	Features  string    `db:"features" json:"features"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type UserOrganization struct {
	ID             string    `db:"id" json:"id"`
	UserID         string    `db:"user_id" json:"userId"`
	OrganizationID string    `db:"organization_id" json:"organizationId"`
	Role           string    `db:"role" json:"role"`
	Permissions    string    `db:"permissions" json:"permissions"`
	JoinedAt       time.Time `db:"joined_at" json:"joinedAt"`
}

type PendingInvitation struct {
	ID             string    `db:"id"`
	OrganizationID string    `db:"organization_id"`
	InvitedBy      string    `db:"invited_by"`
	Email          string    `db:"email"`
	Role           string    `db:"role"`
	Token          string    `db:"token"`
	ExpiresAt      time.Time `db:"expires_at"`
	CreatedAt      time.Time `db:"created_at"`
}

type RefreshToken struct {
	ID        string    `db:"id"`
	UserID    string    `db:"user_id"`
	TokenHash string    `db:"token_hash"`
	ExpiresAt time.Time `db:"expires_at"`
	Revoked   bool      `db:"revoked"`
	CreatedAt time.Time `db:"created_at"`
}
