package service

import (
	"context"
	"fmt"

	"github.com/signsafe-io/signsafe-api/internal/model"
	"github.com/signsafe-io/signsafe-api/internal/repository"
)

// OrgService handles organization management operations.
type OrgService struct {
	userRepo *repository.UserRepo
}

// NewOrgService creates a new OrgService.
func NewOrgService(userRepo *repository.UserRepo) *OrgService {
	return &OrgService{userRepo: userRepo}
}

// GetOrganization returns an organization if the requesting user is a member.
func (s *OrgService) GetOrganization(ctx context.Context, userID, orgID string) (*model.Organization, error) {
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.GetOrganization: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("orgService.GetOrganization: access denied")
	}
	org, err := s.userRepo.FindOrganizationByID(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.GetOrganization: %w", err)
	}
	return org, nil
}

// UpdateOrganization updates the org name if the requesting user is a member.
func (s *OrgService) UpdateOrganization(ctx context.Context, userID, orgID, name string) (*model.Organization, error) {
	if name == "" {
		return nil, fmt.Errorf("orgService.UpdateOrganization: name cannot be empty")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return nil, fmt.Errorf("orgService.UpdateOrganization: %w", err)
	}
	if !member {
		return nil, fmt.Errorf("orgService.UpdateOrganization: access denied")
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
		return nil, fmt.Errorf("orgService.ListMembers: access denied")
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

// InviteMember adds a user (looked up by email) to an org.
// The inviting user must already be a member.
func (s *OrgService) InviteMember(ctx context.Context, userID, orgID, email, role string) error {
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("orgService.InviteMember: %w", err)
	}
	if !member {
		return fmt.Errorf("orgService.InviteMember: access denied")
	}
	if role == "" {
		role = "member"
	}
	target, err := s.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("orgService.InviteMember: %w", err)
	}
	if target == nil {
		return fmt.Errorf("orgService.InviteMember: user not found")
	}
	return s.userRepo.AddOrgMember(ctx, target.ID, orgID, role)
}

// RemoveMember removes a member from the org.
// The requesting user must be a member; they cannot remove themselves.
func (s *OrgService) RemoveMember(ctx context.Context, userID, orgID, targetUserID string) error {
	if userID == targetUserID {
		return fmt.Errorf("orgService.RemoveMember: cannot remove yourself")
	}
	member, err := s.userRepo.IsOrgMember(ctx, userID, orgID)
	if err != nil {
		return fmt.Errorf("orgService.RemoveMember: %w", err)
	}
	if !member {
		return fmt.Errorf("orgService.RemoveMember: access denied")
	}
	return s.userRepo.RemoveOrgMember(ctx, targetUserID, orgID)
}
