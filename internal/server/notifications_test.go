package server

import (
	"testing"
	"time"
)

func TestNotificationEveryN_Linear(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)

	if got := notificationEveryN(start, start, end, 4); got != 1 {
		t.Fatalf("expected everyN=1 at start, got %d", got)
	}

	mid := start.Add(15 * time.Minute)
	if got := notificationEveryN(mid, start, end, 4); got != 2 {
		t.Fatalf("expected everyN=2 at mid, got %d", got)
	}

	late := start.Add(25 * time.Minute)
	if got := notificationEveryN(late, start, end, 4); got != 3 {
		t.Fatalf("expected everyN=3 near end, got %d", got)
	}

	if got := notificationEveryN(end, start, end, 4); got != 4 {
		t.Fatalf("expected everyN=4 at end, got %d", got)
	}
}
