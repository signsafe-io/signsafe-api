package model

import "time"

type AuditEvent struct {
	ID             string    `db:"id" json:"id"`
	ActorID        *string   `db:"actor_id" json:"actorId"`
	ActorEmail     *string   `db:"actor_email" json:"actorEmail"`
	Action         string    `db:"action" json:"action"`
	TargetType     *string   `db:"target_type" json:"targetType"`
	TargetID       *string   `db:"target_id" json:"targetId"`
	OrganizationID *string   `db:"organization_id" json:"organizationId"`
	Context        string    `db:"context" json:"context"`
	IPAddress      *string   `db:"ip_address" json:"ipAddress"`
	UserAgent      *string   `db:"user_agent" json:"userAgent"`
	CreatedAt      time.Time `db:"created_at" json:"createdAt"`
}
