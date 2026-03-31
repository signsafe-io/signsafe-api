package service

import (
	"context"
	"fmt"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/email"
	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/repository"
	"github.com/signsafe-io/signsafe-api/internal/util"
)

const invitationTTL = 7 * 24 * time.Hour

// OrgService handles organization management operations.
type OrgService struct {
	userRepo    *repository.UserRepo
	emailClient *email.Client
	appURL      string
}

// NewOrgService creates a new OrgService.
func NewOrgService(userRepo *repository.UserRepo, emailClient *email.Client, appURL string) *OrgService {
	return &OrgService{userRepo: userRepo, emailClient: emailClient, appURL: appURL}
}

// OrganizationSummary is the public view of an organization with the user's role.
type OrganizationSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Plan string `json:"plan"`
	Role string `json:"role"`
}

// ListMyOrganizations returns all organizations that the given user belongs to.
func (s *OrgService) ListMyOrganizations(ctx context.Context, userID string) ([]OrganizationSummary, error) {
	rows, err := s.userRepo.ListUserOrganizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("orgService.ListMyOrganizations: %w", err)
	}
	out := make([]OrganizationSummary, len(rows))
	for i, row := range rows {
		out[i] = OrganizationSummary{
			ID:   row.ID,
			Name: row.Name,
			Plan: row.Plan,
			Role: row.Role,
		}
	}
	return out, nil
}

// CreateOrganization creates a new organization and adds the requesting user as admin.
func (s *OrgService) CreateOrganization(ctx context.Context, userID, name string) (*model.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("orgService.CreateOrganization: %w: name cannot be empty", ErrInvalidInput)
	}
	org := &model.Organization{
		ID:       util.NewID(),
		Name:     name,
		Plan:     "free",
		Features: "[]",
	}
	if err := s.userRepo.CreateOrganizationWithAdmin(ctx, org, userID); err != nil {
		return nil, fmt.Errorf("orgService.CreateOrganization: %w", err)
	}
	return org, nil
}

// GetOrganization returns an organization if the requesting user is a member.
func (s *OrgService) GetOrganization(ctx context.Context, userID, orgID string) (*model.Organization, error) {
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.GetOrganization: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("orgService.GetOrganization: %w", ErrAccessDenied)
	}
	org, err := s.userRepo.FindOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.GetOrganization: %w", err)
	}
	return org, nil
}

// UpdateOrganization updates the org name if the requesting user is an admin.
func (s *OrgService) UpdateOrganization(ctx context.Context, userID, orgID, name string) (*model.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("orgService.UpdateOrganization: %w: name cannot be empty", ErrInvalidInput)
	}
	role, err := s.userRepo.GetMemberRole(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.UpdateOrganization: %w", err)
	}
	if role != "admin" {
		return nil, fmt.Errorf("orgService.UpdateOrganization: %w", ErrAccessDenied)
	}
	org, err := s.userRepo.UpdateOrganizationName(ctx, orgID, name)
	if err != nil {
		return nil, fmt.Errorf("orgService.UpdateOrganization: %w", err)
	}
	if org == nil {
		return nil, fmt.Errorf("orgService.UpdateOrganization: organization not found")
	}
	return org, nil
}

// MemberInfo is the public view of a member.
type MemberInfo struct {
	UserID   string `json:"userId"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Role     string `json:"role"`
	JoinedAt string `json:"joinedAt"`
}

// ListMembers returns members of an org if the requesting user is a member.
func (s *OrgService) ListMembers(ctx context.Context, userID, orgID string) ([]MemberInfo, error) {
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.ListMembers: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("orgService.ListMembers: %w", ErrAccessDenied)
	}
	rows, err := s.userRepo.ListOrgMembers(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.ListMembers: %w", err)
	}
	out := make([]MemberInfo, len(rows))
	for i, m := range rows {
		out[i] = MemberInfo{
			UserID:   m.UserID,
			Email:    m.Email,
			FullName: m.FullName,
			Role:     m.Role,
			JoinedAt: m.JoinedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	return out, nil
}

// InviteMember adds or invites a user to an org.
// If the user already has an account, they are added immediately.
// If not, an invitation email is sent and the invite is stored pending signup.
func (s *OrgService) InviteMember(ctx context.Context, userID, orgID, inviteeEmail, role string) error {
	requesterRole, err := s.userRepo.GetMemberRole(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("orgService.InviteMember: %w", err)
	}
	if requesterRole != "admin" {
		return fmt.Errorf("orgService.InviteMember: %w", ErrAccessDenied)
	}
	if role == "" {
		role = "member"
	}
	validRoles := map[string]bool{"admin": true, "member": true, "reviewer": true}
	if !validRoles[role] {
		return fmt.Errorf("orgService.InviteMember: %w: %q is not a valid role", ErrInvalidInput, role)
	}

	// Check if this user already exists.
	target, err := s.userRepo.FindByEmail(ctx, inviteeEmail)
	if err != nil {
		return fmt.Errorf("orgService.InviteMember: %w", err)
	}
	if target != nil {
		// Already a SignSafe user — add directly.
		return s.userRepo.AddOrgMember(ctx, target.ID, orgID, role)
	}

	// Not yet a user — create a pending invitation and send email.
	inviter, err := s.userRepo.FindByID(ctx, userID)
	if err != nil || inviter == nil {
		return fmt.Errorf("orgService.InviteMember: inviter not found")
	}
	org, err := s.userRepo.FindOrganizationByID(ctx, orgID)
	if err != nil || org == nil {
		return fmt.Errorf("orgService.InviteMember: org not found")
	}

	token := generateOpaqueToken()
	inv := &model.PendingInvitation{
		ID:             util.NewID(),
		OrganizationID: orgID,
		InvitedBy:      userID,
		Email:          inviteeEmail,
		Role:           role,
		Token:          token,
		ExpiresAt:      time.Now().Add(invitationTTL),
	}
	if err := s.userRepo.CreateInvitation(ctx, inv); err != nil {
		return fmt.Errorf("orgService.InviteMember: %w", err)
	}

	signupURL := fmt.Sprintf("%s/signup?invite=%s", s.appURL, token)
	if err := s.emailClient.SendInvitationEmail(inviteeEmail, org.Name, inviter.FullName, signupURL); err != nil {
		// Non-fatal: invitation is stored; email delivery failure shouldn't block the operation.
		_ = err
	}
	return nil
}

// AcceptInvitation adds the newly signed-up user to any pending invitations.
// Called after a successful signup.
func (s *OrgService) AcceptPendingInvitations(ctx context.Context, userID, email string) {
	invs, err := s.userRepo.FindInvitationsByEmail(ctx, email)
	if err != nil || len(invs) == 0 {
		return
	}
	for _, inv := range invs {
		if err := s.userRepo.AddOrgMember(ctx, userID, inv.OrganizationID, inv.Role); err == nil {
			_ = s.userRepo.DeleteInvitation(ctx, inv.ID)
		}
	}
}

// RemoveMember removes a member from the org.
func (s *OrgService) RemoveMember(ctx context.Context, userID, orgID, targetUserID string) error {
	if userID == targetUserID {
		return fmt.Errorf("orgService.RemoveMember: %w: cannot remove yourself", ErrInvalidInput)
	}
	role, err := s.userRepo.GetMemberRole(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("orgService.RemoveMember: %w", err)
	}
	if role != "admin" {
		return fmt.Errorf("orgService.RemoveMember: %w", ErrAccessDenied)
	}
	return s.userRepo.RemoveOrgMember(ctx, targetUserID, orgID)
}

// UpdateMemberRole changes the role of a member in the org.
func (s *OrgService) UpdateMemberRole(ctx context.Context, userID, orgID, targetUserID, role string) error {
	validRoles := map[string]bool{"admin": true, "member": true, "reviewer": true}
	if !validRoles[role] {
		return fmt.Errorf("orgService.UpdateMemberRole: %w: %q is not a valid role", ErrInvalidInput, role)
	}
	requesterRole, err := s.userRepo.GetMemberRole(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("orgService.UpdateMemberRole: %w", err)
	}
	if requesterRole != "admin" {
		return fmt.Errorf("orgService.UpdateMemberRole: %w", ErrAccessDenied)
	}
	return s.userRepo.UpdateMemberRole(ctx, targetUserID, orgID, role)
}
