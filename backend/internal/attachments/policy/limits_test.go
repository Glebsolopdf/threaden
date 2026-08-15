package policy_test

import (
	"testing"
	"time"

	"voice-rooms/internal/attachments"
)

func TestEffectiveLimitsRestrictNewAccountsForFirstDay(t *testing.T) {
	base := attachments.Limits{MaxUserStoredBytes: 50 * 1024 * 1024, MaxUserDailyBytes: 20 * 1024 * 1024}
	now := time.Unix(100, 0).UTC()
	newAccount := attachments.EffectiveLimits(base, now.Add(-23*time.Hour), now)
	if newAccount.MaxUserStoredBytes != 20*1024*1024 || newAccount.MaxUserDailyBytes != 20*1024*1024 {
		t.Fatalf("new account limits = %+v", newAccount)
	}
	oldAccount := attachments.EffectiveLimits(base, now.Add(-24*time.Hour), now)
	if oldAccount.MaxUserStoredBytes != 50*1024*1024 || oldAccount.MaxUserDailyBytes != 0 {
		t.Fatalf("old account limits = %+v", oldAccount)
	}
}
