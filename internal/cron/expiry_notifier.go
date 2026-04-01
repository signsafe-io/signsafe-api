// Package cron contains scheduled background jobs.
package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/signsafe-io/signsafe-api/internal/cache"
	"github.com/signsafe-io/signsafe-api/internal/email"
	"github.com/signsafe-io/signsafe-api/internal/repository"
)

// ExpiryNotifier sends expiry-alert emails to org admins on a daily schedule.
type ExpiryNotifier struct {
	userRepo     *repository.UserRepo
	contractRepo *repository.ContractRepo
	cacheClient  *cache.Client
	emailClient  *email.Client
	alertDays    int
}

// NewExpiryNotifier creates an ExpiryNotifier.
// alertDays controls how far in advance to warn (e.g. 30 means "expiring within 30 days").
func NewExpiryNotifier(
	userRepo *repository.UserRepo,
	contractRepo *repository.ContractRepo,
	cacheClient *cache.Client,
	emailClient *email.Client,
	alertDays int,
) *ExpiryNotifier {
	if alertDays <= 0 {
		alertDays = 30
	}
	return &ExpiryNotifier{
		userRepo:     userRepo,
		contractRepo: contractRepo,
		cacheClient:  cacheClient,
		emailClient:  emailClient,
		alertDays:    alertDays,
	}
}

// Start launches the daily notification loop. It blocks until ctx is cancelled.
// The first run fires at the next 00:00 KST; subsequent runs fire every 24 h.
func (n *ExpiryNotifier) Start(ctx context.Context) {
	nextRun := nextMidnightKST()
	slog.Info("expiry notifier: scheduled", "first_run", nextRun.Format(time.RFC3339))

	timer := time.NewTimer(time.Until(nextRun))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("expiry notifier: shutting down")
			return
		case <-timer.C:
			n.run(ctx)
			// Schedule next run 24 h later.
			timer.Reset(24 * time.Hour)
		}
	}
}

// RunOnce executes the notification job synchronously (useful for testing / forced runs).
func (n *ExpiryNotifier) RunOnce(ctx context.Context) {
	n.run(ctx)
}

// run fetches all organizations and sends alerts for those with expiring contracts.
func (n *ExpiryNotifier) run(ctx context.Context) {
	slog.Info("expiry notifier: run started", "alert_days", n.alertDays)

	orgs, err := n.userRepo.ListAllOrganizations(ctx)
	if err != nil {
		slog.Error("expiry notifier: list orgs failed", "error", err)
		return
	}

	today := time.Now().UTC().Format("2006-01-02")
	sent, skipped, failed := 0, 0, 0

	for _, org := range orgs {
		contracts, err := n.contractRepo.ListExpiringContracts(ctx, org.ID, n.alertDays)
		if err != nil {
			slog.Warn("expiry notifier: list contracts failed", "org_id", org.ID, "error", err)
			failed++
			continue
		}
		if len(contracts) == 0 {
			skipped++
			continue
		}

		// De-duplicate: only send once per org per calendar day.
		dedupKey := fmt.Sprintf("expiry-notified:%s:%s", org.ID, today)
		ok, err := n.cacheClient.SetNX(ctx, dedupKey, "1", 25*time.Hour)
		if err != nil {
			slog.Warn("expiry notifier: dedup check failed", "org_id", org.ID, "error", err)
			// Fall through and attempt to send anyway to avoid silent skips.
		} else if !ok {
			slog.Debug("expiry notifier: already sent today", "org_id", org.ID)
			skipped++
			continue
		}

		// Convert model.Contract slice to ExpiringContractInfo slice.
		infos := make([]email.ExpiringContractInfo, 0, len(contracts))
		for _, c := range contracts {
			if c.ExpiresAt == nil {
				continue
			}
			infos = append(infos, email.ExpiringContractInfo{
				Title:     c.Title,
				ExpiresAt: *c.ExpiresAt,
			})
		}
		if len(infos) == 0 {
			skipped++
			continue
		}

		// Fetch admin emails.
		admins, err := n.userRepo.ListOrgAdmins(ctx, org.ID)
		if err != nil || len(admins) == 0 {
			slog.Warn("expiry notifier: no admins found", "org_id", org.ID, "error", err)
			skipped++
			continue
		}

		for _, admin := range admins {
			if err := n.emailClient.SendExpiryAlertEmail(admin.Email, org.Name, infos, n.alertDays); err != nil {
				slog.Warn("expiry notifier: send failed", "to", admin.Email, "org_id", org.ID, "error", err)
				failed++
			} else {
				slog.Info("expiry notifier: alert sent", "to", admin.Email, "org", org.Name, "contracts", len(infos))
				sent++
			}
		}
	}

	slog.Info("expiry notifier: run complete", "sent", sent, "skipped", skipped, "failed", failed)
}

// nextMidnightKST returns the next 00:00:00 in Asia/Seoul (KST = UTC+9).
// Falls back to UTC+9 fixed offset if the timezone database is unavailable.
func nextMidnightKST() time.Time {
	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		loc = time.FixedZone("KST", 9*60*60)
	}
	now := time.Now().In(loc)
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, loc)
	return midnight
}
