package server

import (
	"context"
	"testing"
	"time"

	"tronbyt-server/internal/data"

	"gorm.io/gorm"
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

func TestUpdateNotificationInterstitial_AllowsEndBoundary(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "notif-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := data.Device{ID: "notif-device", Username: user.Username}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	apps := []data.App{
		{DeviceID: device.ID, Iname: "app-1", Name: "App 1", Enabled: true, Pushed: true, Order: 1},
		{DeviceID: device.ID, Iname: "app-2", Name: "App 2", Enabled: true, Pushed: true, Order: 2},
		{DeviceID: device.ID, Iname: "app-3", Name: "App 3", Enabled: true, Pushed: true, Order: 3},
	}
	for i := range apps {
		if err := gorm.G[data.App](s.DB).Create(ctx, &apps[i]); err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(10 * time.Minute)

	notification := data.DeviceNotification{
		ID:                "notif-1",
		DeviceID:          device.ID,
		InterstitialUntil: &end,
		CreatedAt:         start,
	}
	if err := gorm.G[data.DeviceNotification](s.DB).Create(ctx, &notification); err != nil {
		t.Fatalf("failed to create notification: %v", err)
	}

	every1 := 1
	override := data.DeviceOverride{
		ID:             "ov-1",
		DeviceID:       device.ID,
		Kind:           data.OverrideInterstitial,
		EveryN:         &every1,
		ManagedByNotif: &notification.ID,
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &override); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}

	s.updateNotificationInterstitial(ctx, &d, &notification, end)

	updated, err := gorm.G[data.DeviceOverride](s.DB).Where("id = ?", override.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to load override: %v", err)
	}
	if updated.EveryN == nil || *updated.EveryN != 2 {
		t.Fatalf("expected everyN=2 at end boundary, got %v", updated.EveryN)
	}
}
