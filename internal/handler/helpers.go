package handler

import (
	"context"
	"net"
	"net/http"

	"github.com/signsafe-io/signsafe-api/internal/middleware"
	"github.com/signsafe-io/signsafe-api/internal/service"
)

// clientIP extracts the real client IP from the request, stripping the port
// so the value can be stored in a PostgreSQL INET column.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// logAuditEvent records an audit event in a best-effort, non-blocking way.
// A failure to write the audit log never blocks or fails the original request.
func logAuditEvent(r *http.Request, auditSvc *service.AuditService, action string, targetType, targetID, orgID *string) {
	ip := clientIP(r)
	ua := r.UserAgent()
	userID := middleware.UserIDFromContext(r.Context())

	req := service.CreateAuditEventRequest{
		Action:         action,
		TargetType:     targetType,
		TargetID:       targetID,
		OrganizationID: orgID,
		IPAddress:      &ip,
		UserAgent:      &ua,
	}
	if userID != "" {
		req.ActorID = &userID
	}
	// Fire-and-forget: use context.Background() so the write is not
	// cancelled when the HTTP request context is done.
	go func() {
		_, _ = auditSvc.CreateAuditEvent(context.Background(), req)
	}()
}
