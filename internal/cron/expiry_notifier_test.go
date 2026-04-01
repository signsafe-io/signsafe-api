package cron

import (
	"testing"
	"time"
)

func TestNextMidnightKST(t *testing.T) {
	got := nextMidnightKST()

	loc, err := time.LoadLocation("Asia/Seoul")
	if err != nil {
		loc = time.FixedZone("KST", 9*60*60)
	}

	midnight := got.In(loc)

	if midnight.Hour() != 0 || midnight.Minute() != 0 || midnight.Second() != 0 {
		t.Errorf("nextMidnightKST returned %v, want 00:00:00 KST", midnight)
	}

	if got.Before(time.Now()) {
		t.Errorf("nextMidnightKST returned a time in the past: %v", got)
	}
}

func TestNextMidnightKSTIsInFuture(t *testing.T) {
	now := time.Now()
	next := nextMidnightKST()
	if !next.After(now) {
		t.Errorf("expected next midnight KST to be after now, got %v", next)
	}
	// Should be at most ~24h in the future.
	maxFuture := now.Add(25 * time.Hour)
	if next.After(maxFuture) {
		t.Errorf("next midnight KST is more than 25h away: %v", next)
	}
}
