package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tronbyt-server/internal/data"

	"gorm.io/gorm"
)

func TestGetOverrideImage_RemainingShowsAndCleanup(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "override-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "override-device", Username: user.Username, Brightness: 10}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	shows := 2
	ov := data.DeviceOverride{
		ID:             "override-1",
		DeviceID:       device.ID,
		Kind:           data.OverrideForeground,
		RemainingShows: &shows,
		ImageKey:       "override-1",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}
	img := []byte("override-image")
	if err := s.saveOverrideImage(device.ID, ov.ImageKey, img); err != nil {
		t.Fatalf("failed to save override image: %v", err)
	}

	got, gotOv, err := s.getOverrideImage(ctx, device.ID, data.OverrideForeground, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed: %v", err)
	}
	if gotOv == nil || gotOv.ID != ov.ID {
		t.Fatalf("expected override %s, got %v", ov.ID, gotOv)
	}
	if string(got) != string(img) {
		t.Fatalf("expected override image %q, got %q", img, got)
	}

	updated, err := gorm.G[data.DeviceOverride](s.DB).Where("id = ?", ov.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to load override: %v", err)
	}
	if updated.RemainingShows == nil || *updated.RemainingShows != 1 {
		t.Fatalf("expected remainingShows=1, got %v", updated.RemainingShows)
	}

	got, gotOv, err = s.getOverrideImage(ctx, device.ID, data.OverrideForeground, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed (second): %v", err)
	}
	if gotOv == nil || gotOv.ID != ov.ID {
		t.Fatalf("expected override %s on second fetch, got %v", ov.ID, gotOv)
	}
	if string(got) != string(img) {
		t.Fatalf("expected override image %q on second fetch, got %q", img, got)
	}

	if _, err := gorm.G[data.DeviceOverride](s.DB).Where("id = ?", ov.ID).First(ctx); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected override to be deleted, got err=%v", err)
	}
	path, err := s.overrideImagePath(device.ID, ov.ImageKey)
	if err != nil {
		t.Fatalf("failed to resolve override image path: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected override image to be deleted, got err=%v", err)
	}
}

func TestGetOverrideImage_MissingImageDeletesOverride(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "missing-image-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "missing-image-device", Username: user.Username}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	ov := data.DeviceOverride{
		ID:       "override-missing",
		DeviceID: device.ID,
		Kind:     data.OverridePinned,
		ImageKey: "override-missing",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}

	img, gotOv, err := s.getOverrideImage(ctx, device.ID, data.OverridePinned, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed: %v", err)
	}
	if img != nil || gotOv != nil {
		t.Fatalf("expected no override image, got %v / %v", img, gotOv)
	}
	if _, err := gorm.G[data.DeviceOverride](s.DB).Where("id = ?", ov.ID).First(ctx); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected override to be deleted, got err=%v", err)
	}
}

func TestGetNextAppImage_OverridePrecedence(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "precedence-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "precedence-device", Username: user.Username, Brightness: 10}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	pinned := data.DeviceOverride{
		ID:             "override-pinned",
		DeviceID:       device.ID,
		Kind:           data.OverridePinned,
		DisplayTimeSec: intPtr(8),
		ImageKey:       "override-pinned",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &pinned); err != nil {
		t.Fatalf("failed to create pinned override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, pinned.ImageKey, []byte("pinned-img")); err != nil {
		t.Fatalf("failed to save pinned override image: %v", err)
	}

	remaining := 1
	foreground := data.DeviceOverride{
		ID:             "override-foreground",
		DeviceID:       device.ID,
		Kind:           data.OverrideForeground,
		DisplayTimeSec: intPtr(5),
		RemainingShows: &remaining,
		ImageKey:       "override-foreground",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &foreground); err != nil {
		t.Fatalf("failed to create foreground override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, foreground.ImageKey, []byte("foreground-img")); err != nil {
		t.Fatalf("failed to save foreground override image: %v", err)
	}

	img, app, err := s.GetNextAppImage(ctx, &device, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed: %v", err)
	}
	if string(img) != "foreground-img" {
		t.Fatalf("expected foreground override image, got %q", img)
	}
	if app == nil || app.DisplayTime != 5 {
		t.Fatalf("expected displayTime=5, got %v", app)
	}

	img, app, err = s.GetNextAppImage(ctx, &device, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed (second): %v", err)
	}
	if string(img) != "pinned-img" {
		t.Fatalf("expected pinned override image, got %q", img)
	}
	if app == nil || app.DisplayTime != 8 {
		t.Fatalf("expected displayTime=8, got %v", app)
	}
}

func TestGetNextAppImage_NightModeMinPriority(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "night-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := data.Device{
		ID:               "night-device",
		Username:         user.Username,
		Brightness:       10,
		NightModeEnabled: true,
		NightStart:       "00:00",
		NightEnd:         "23:59",
		NightBrightness:  10,
		LastAppIndex:     -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	app := data.App{
		DeviceID: device.ID,
		Iname:    "app-1",
		Name:     "App 1",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	deviceWebpDir, err := s.ensureDeviceImageDir(device.ID)
	if err != nil {
		t.Fatalf("failed to create device webp dir: %v", err)
	}
	pushedDir := filepath.Join(deviceWebpDir, "pushed")
	if err := os.MkdirAll(pushedDir, 0755); err != nil {
		t.Fatalf("failed to create pushed dir: %v", err)
	}
	appImage := []byte("app-image")
	if err := os.WriteFile(filepath.Join(pushedDir, "app-1.webp"), appImage, 0644); err != nil {
		t.Fatalf("failed to write app image: %v", err)
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}

	low := data.DeviceOverride{
		ID:       "override-low",
		DeviceID: device.ID,
		Kind:     data.OverridePinned,
		Priority: 50,
		ImageKey: "override-low",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &low); err != nil {
		t.Fatalf("failed to create low override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, low.ImageKey, []byte("low-image")); err != nil {
		t.Fatalf("failed to save low override image: %v", err)
	}

	img, appOut, err := s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed: %v", err)
	}
	if string(img) != string(appImage) {
		t.Fatalf("expected app image when priority too low, got %q", img)
	}
	if appOut == nil || appOut.Iname != "app-1" {
		t.Fatalf("expected app-1 when priority too low, got %v", appOut)
	}

	high := data.DeviceOverride{
		ID:             "override-high",
		DeviceID:       device.ID,
		Kind:           data.OverridePinned,
		Priority:       100,
		DisplayTimeSec: intPtr(9),
		ImageKey:       "override-high",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &high); err != nil {
		t.Fatalf("failed to create high override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, high.ImageKey, []byte("high-image")); err != nil {
		t.Fatalf("failed to save high override image: %v", err)
	}

	img, appOut, err = s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed (high): %v", err)
	}
	if string(img) != "high-image" {
		t.Fatalf("expected high-priority override image, got %q", img)
	}
	if appOut == nil || appOut.DisplayTime != 9 {
		t.Fatalf("expected displayTime=9 for override, got %v", appOut)
	}
}

func TestOverrideDisplayTime_DefaultsToDeviceInterval(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "default-dwell-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{
		ID:              "default-dwell-device",
		Username:        user.Username,
		Brightness:      10,
		DefaultInterval: 17,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	ov := data.DeviceOverride{
		ID:       "override-default-dwell",
		DeviceID: device.ID,
		Kind:     data.OverridePinned,
		ImageKey: "override-default-dwell",
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, ov.ImageKey, []byte("img")); err != nil {
		t.Fatalf("failed to save override image: %v", err)
	}

	img, app, err := s.GetNextAppImage(ctx, &device, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed: %v", err)
	}
	if string(img) != "img" {
		t.Fatalf("expected override image, got %q", img)
	}
	if app == nil {
		t.Fatalf("expected override app, got nil")
	}
	if app.DisplayTime != 0 {
		t.Fatalf("expected displayTime=0 for default dwell, got %d", app.DisplayTime)
	}
	if dwell := device.GetEffectiveDwellTime(app); dwell != 17 {
		t.Fatalf("expected dwell=17, got %d", dwell)
	}
}

func TestGetOverrideImage_ForegroundQueueOrder(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "foreground-queue-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "foreground-queue-device", Username: user.Username}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	now := time.Now()
	start := now.Add(-2 * time.Minute)
	shows := 1

	ov1 := data.DeviceOverride{
		ID:             "fg-1",
		DeviceID:       device.ID,
		Kind:           data.OverrideForeground,
		Priority:       0,
		StartsAt:       &start,
		RemainingShows: &shows,
		ImageKey:       "fg-1",
		CreatedAt:      now.Add(-2 * time.Minute),
	}
	ov2 := data.DeviceOverride{
		ID:             "fg-2",
		DeviceID:       device.ID,
		Kind:           data.OverrideForeground,
		Priority:       0,
		StartsAt:       &start,
		RemainingShows: &shows,
		ImageKey:       "fg-2",
		CreatedAt:      now.Add(-1 * time.Minute),
	}

	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov1); err != nil {
		t.Fatalf("failed to create override 1: %v", err)
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov2); err != nil {
		t.Fatalf("failed to create override 2: %v", err)
	}

	if err := s.saveOverrideImage(device.ID, ov1.ImageKey, []byte("img-1")); err != nil {
		t.Fatalf("failed to save override image 1: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, ov2.ImageKey, []byte("img-2")); err != nil {
		t.Fatalf("failed to save override image 2: %v", err)
	}

	img, _, err := s.getOverrideImage(ctx, device.ID, data.OverrideForeground, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed: %v", err)
	}
	if string(img) != "img-1" {
		t.Fatalf("expected first override image, got %q", img)
	}

	img, _, err = s.getOverrideImage(ctx, device.ID, data.OverrideForeground, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed (second): %v", err)
	}
	if string(img) != "img-2" {
		t.Fatalf("expected second override image, got %q", img)
	}

	if _, err := gorm.G[data.DeviceOverride](s.DB).Where("device_id = ?", device.ID).First(ctx); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected all foreground overrides to be consumed, got err=%v", err)
	}
}

func TestGetOverrideImage_PinnedRoundRobin(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "pinned-roundrobin-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "pinned-roundrobin-device", Username: user.Username}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	now := time.Now()
	start := now.Add(-2 * time.Minute)

	ov1 := data.DeviceOverride{
		ID:        "pin-1",
		DeviceID:  device.ID,
		Kind:      data.OverridePinned,
		Priority:  0,
		StartsAt:  &start,
		ImageKey:  "pin-1",
		CreatedAt: now.Add(-2 * time.Minute),
	}
	ov2 := data.DeviceOverride{
		ID:        "pin-2",
		DeviceID:  device.ID,
		Kind:      data.OverridePinned,
		Priority:  0,
		StartsAt:  &start,
		ImageKey:  "pin-2",
		CreatedAt: now.Add(-1 * time.Minute),
	}

	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov1); err != nil {
		t.Fatalf("failed to create override 1: %v", err)
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov2); err != nil {
		t.Fatalf("failed to create override 2: %v", err)
	}

	if err := s.saveOverrideImage(device.ID, ov1.ImageKey, []byte("pin-1")); err != nil {
		t.Fatalf("failed to save override image 1: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, ov2.ImageKey, []byte("pin-2")); err != nil {
		t.Fatalf("failed to save override image 2: %v", err)
	}

	img, _, err := s.getOverrideImage(ctx, device.ID, data.OverridePinned, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed: %v", err)
	}
	if string(img) != "pin-1" {
		t.Fatalf("expected first pinned override, got %q", img)
	}

	img, _, err = s.getOverrideImage(ctx, device.ID, data.OverridePinned, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed (second): %v", err)
	}
	if string(img) != "pin-2" {
		t.Fatalf("expected second pinned override, got %q", img)
	}

	img, _, err = s.getOverrideImage(ctx, device.ID, data.OverridePinned, 0)
	if err != nil {
		t.Fatalf("getOverrideImage failed (third): %v", err)
	}
	if string(img) != "pin-1" {
		t.Fatalf("expected round-robin to return first pinned override, got %q", img)
	}
}

func TestPeekOverride_InterstitialRoundRobin(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "interstitial-roundrobin-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	device := data.Device{ID: "interstitial-roundrobin-device", Username: user.Username}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	now := time.Now()
	start := now.Add(-2 * time.Minute)

	ov1 := data.DeviceOverride{
		ID:        "int-1",
		DeviceID:  device.ID,
		Kind:      data.OverrideInterstitial,
		Priority:  0,
		StartsAt:  &start,
		ImageKey:  "int-1",
		CreatedAt: now.Add(-2 * time.Minute),
	}
	ov2 := data.DeviceOverride{
		ID:        "int-2",
		DeviceID:  device.ID,
		Kind:      data.OverrideInterstitial,
		Priority:  0,
		StartsAt:  &start,
		ImageKey:  "int-2",
		CreatedAt: now.Add(-1 * time.Minute),
	}

	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov1); err != nil {
		t.Fatalf("failed to create override 1: %v", err)
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov2); err != nil {
		t.Fatalf("failed to create override 2: %v", err)
	}

	selected, err := s.peekOverride(ctx, device.ID, data.OverrideInterstitial, 0)
	if err != nil {
		t.Fatalf("peekOverride failed: %v", err)
	}
	if selected == nil || selected.ID != ov1.ID {
		t.Fatalf("expected first interstitial override, got %v", selected)
	}
	if _, err := s.markOverrideServed(ctx, selected); err != nil {
		t.Fatalf("markOverrideServed failed: %v", err)
	}

	selected, err = s.peekOverride(ctx, device.ID, data.OverrideInterstitial, 0)
	if err != nil {
		t.Fatalf("peekOverride failed (second): %v", err)
	}
	if selected == nil || selected.ID != ov2.ID {
		t.Fatalf("expected second interstitial override, got %v", selected)
	}
	if _, err := s.markOverrideServed(ctx, selected); err != nil {
		t.Fatalf("markOverrideServed failed (second): %v", err)
	}

	selected, err = s.peekOverride(ctx, device.ID, data.OverrideInterstitial, 0)
	if err != nil {
		t.Fatalf("peekOverride failed (third): %v", err)
	}
	if selected == nil || selected.ID != ov1.ID {
		t.Fatalf("expected round-robin to return first interstitial override, got %v", selected)
	}
}

func TestGetNextAppImage_InterstitialOverrideIndexMapping(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "interstitial-index-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := data.Device{
		ID:           "interstitial-index-device",
		Username:     user.Username,
		Brightness:   10,
		LastAppIndex: -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	appA := data.App{
		DeviceID: device.ID,
		Iname:    "app-a",
		Name:     "App A",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	appB := data.App{
		DeviceID: device.ID,
		Iname:    "app-b",
		Name:     "App B",
		Enabled:  true,
		Pushed:   true,
		Order:    2,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appA); err != nil {
		t.Fatalf("failed to create app A: %v", err)
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appB); err != nil {
		t.Fatalf("failed to create app B: %v", err)
	}

	deviceWebpDir, err := s.ensureDeviceImageDir(device.ID)
	if err != nil {
		t.Fatalf("failed to create device webp dir: %v", err)
	}
	pushedDir := filepath.Join(deviceWebpDir, "pushed")
	if err := os.MkdirAll(pushedDir, 0755); err != nil {
		t.Fatalf("failed to create pushed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pushedDir, "app-a.webp"), []byte("A"), 0644); err != nil {
		t.Fatalf("failed to write app A image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pushedDir, "app-b.webp"), []byte("B"), 0644); err != nil {
		t.Fatalf("failed to write app B image: %v", err)
	}

	now := time.Now()
	start := now.Add(-1 * time.Minute)
	ov := data.DeviceOverride{
		ID:        "interstitial-override",
		DeviceID:  device.ID,
		Kind:      data.OverrideInterstitial,
		Priority:  0,
		StartsAt:  &start,
		ImageKey:  "interstitial-override",
		CreatedAt: now,
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, ov.ImageKey, []byte("OVERRIDE")); err != nil {
		t.Fatalf("failed to save override image: %v", err)
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}

	_, app, err := s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed: %v", err)
	}
	if app == nil || app.Iname != "app-a" {
		t.Fatalf("expected app A first, got %v", app)
	}

	img, app, err := s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed (override): %v", err)
	}
	if app == nil || app.DisplayTime != 0 {
		t.Fatalf("expected override app placeholder, got %v", app)
	}
	if string(img) != "OVERRIDE" {
		t.Fatalf("expected override image, got %q", img)
	}

	s.deleteOverride(ctx, &ov)

	d, err = gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device after override: %v", err)
	}

	_, app, err = s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed after override: %v", err)
	}
	if app == nil || app.Iname != "app-b" {
		t.Fatalf("expected app B after override, got %v", app)
	}
}

func TestForegroundOverride_DoesNotResetRotation(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "foreground-rotation-user"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := data.Device{
		ID:           "foreground-rotation-device",
		Username:     user.Username,
		Brightness:   10,
		LastAppIndex: -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	appA := data.App{
		DeviceID: device.ID,
		Iname:    "app-a",
		Name:     "App A",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	appB := data.App{
		DeviceID: device.ID,
		Iname:    "app-b",
		Name:     "App B",
		Enabled:  true,
		Pushed:   true,
		Order:    2,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appA); err != nil {
		t.Fatalf("failed to create app A: %v", err)
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appB); err != nil {
		t.Fatalf("failed to create app B: %v", err)
	}

	deviceWebpDir, err := s.ensureDeviceImageDir(device.ID)
	if err != nil {
		t.Fatalf("failed to create device webp dir: %v", err)
	}
	pushedDir := filepath.Join(deviceWebpDir, "pushed")
	if err := os.MkdirAll(pushedDir, 0755); err != nil {
		t.Fatalf("failed to create pushed dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pushedDir, "app-a.webp"), []byte("A"), 0644); err != nil {
		t.Fatalf("failed to write app A image: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pushedDir, "app-b.webp"), []byte("B"), 0644); err != nil {
		t.Fatalf("failed to write app B image: %v", err)
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}

	_, app, err := s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed: %v", err)
	}
	if app == nil || app.Iname != "app-a" {
		t.Fatalf("expected app A first, got %v", app)
	}

	now := time.Now()
	shows := 3
	ov := data.DeviceOverride{
		ID:             "foreground-override",
		DeviceID:       device.ID,
		Kind:           data.OverrideForeground,
		RemainingShows: &shows,
		StartsAt:       &now,
		ImageKey:       "foreground-override",
		CreatedAt:      now,
	}
	if err := gorm.G[data.DeviceOverride](s.DB).Create(ctx, &ov); err != nil {
		t.Fatalf("failed to create override: %v", err)
	}
	if err := s.saveOverrideImage(device.ID, ov.ImageKey, []byte("OVR")); err != nil {
		t.Fatalf("failed to save override image: %v", err)
	}

	for i := 0; i < 3; i++ {
		img, overrideApp, err := s.GetNextAppImage(ctx, &d, &user)
		if err != nil {
			t.Fatalf("GetNextAppImage failed (override %d): %v", i, err)
		}
		if string(img) != "OVR" {
			t.Fatalf("expected override image, got %q", img)
		}
		if overrideApp == nil {
			t.Fatalf("expected override app placeholder, got nil")
		}
	}

	dbDevice, err := gorm.G[data.Device](s.DB).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device after override: %v", err)
	}
	if dbDevice.LastAppIndex != 0 {
		t.Fatalf("expected LastAppIndex to remain on app A (0), got %d", dbDevice.LastAppIndex)
	}

	d, err = gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device after override: %v", err)
	}

	_, app, err = s.GetNextAppImage(ctx, &d, &user)
	if err != nil {
		t.Fatalf("GetNextAppImage failed after override: %v", err)
	}
	if app == nil || app.Iname != "app-b" {
		t.Fatalf("expected app B after override, got %v", app)
	}
}
