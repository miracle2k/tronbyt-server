package server

import (
	"context"
	"testing"
	"time"

	"tronbyt-server/internal/data"

	"gorm.io/gorm"
)

func TestDetermineNextApp_NightMode(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	// Create a user
	user := data.User{Username: "testuser"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create a device with Night Mode enabled all day
	device := data.Device{
		ID:               "device1",
		Username:         user.Username,
		NightModeEnabled: true,
		NightStart:       "00:00",
		NightEnd:         "23:59",
		NightModeApp:     "app-night",
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	// Create Regular App
	appRegular := data.App{
		DeviceID: device.ID,
		Iname:    "app-regular",
		Name:     "Regular App",
		Enabled:  true,
		Pushed:   true, // Bypass rendering check
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appRegular); err != nil {
		t.Fatalf("failed to create regular app: %v", err)
	}

	// Create Night Mode App
	appNight := data.App{
		DeviceID: device.ID,
		Iname:    "app-night",
		Name:     "Night App",
		Enabled:  false, // Usually disabled for day rotation
		Pushed:   true,  // Bypass rendering check
		Order:    2,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appNight); err != nil {
		t.Fatalf("failed to create night app: %v", err)
	}

	// Reload device with apps
	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	// Helper to check what app is selected
	// We want to ensure that ONLY appNight is selected.
	// Since determineNextApp rotates based on LastAppIndex, we should try a few times.

	// Start from index -1 (so next is 0)
	d.LastAppIndex = -1

	for i := range 10 {
		app, _, nextIndex, _, err := s.determineNextApp(ctx, &d, &user, nil)
		if err != nil {
			t.Fatalf("determineNextApp failed: %v", err)
		}
		if app == nil {
			t.Fatalf("expected an app, got nil")
			return
		}

		if app.Iname != "app-night" {
			t.Errorf("Iteration %d: Expected Night Mode app (app-night), got %s", i, app.Iname)
		}

		// Update LastAppIndex for next iteration to simulate rotation
		d.LastAppIndex = nextIndex
	}
}

func TestDetermineNextApp_NightMode_NoAppSelected(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	// Create a user
	user := data.User{Username: "testuser"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create a device with Night Mode enabled all day but NO NightModeApp selected
	device := data.Device{
		ID:               "device2",
		Username:         user.Username,
		NightModeEnabled: true,
		NightStart:       "00:00",
		NightEnd:         "23:59",
		NightModeApp:     "", // Empty!
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	// Create Regular App
	appRegular := data.App{
		DeviceID: device.ID,
		Iname:    "app-regular",
		Name:     "Regular App",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &appRegular); err != nil {
		t.Fatalf("failed to create regular app: %v", err)
	}

	// Reload device with apps
	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	d.LastAppIndex = -1

	// Should return the regular app
	app, selectedOverride, nextIndex, gapIndex, err := s.determineNextApp(ctx, &d, &user, nil)
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if app == nil {
		t.Fatal("expected an app (fallback to rotation), got nil")
		return
	}

	if app.Iname != "app-regular" {
		t.Errorf("Expected regular app (app-regular), got %s", app.Iname)
	}
}

func TestDetermineNextApp_NightModePrecedence(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "testuser_precedence"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	pinnedAppID := "app-pinned"
	nightAppID := "app-night"

	// Create a device with both Pinned App and Night Mode App
	device := data.Device{
		ID:               "device_precedence",
		Username:         user.Username,
		PinnedApp:        &pinnedAppID,
		NightModeEnabled: true,
		NightStart:       "00:00", // Always active
		NightEnd:         "23:59",
		NightModeApp:     nightAppID,
		LastAppIndex:     -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	// Create Pinned App
	if err := gorm.G[data.App](s.DB).Create(ctx, &data.App{
		DeviceID: device.ID,
		Iname:    pinnedAppID,
		Name:     "Pinned App",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}); err != nil {
		t.Fatalf("failed to create pinned app: %v", err)
	}

	// Create Night Mode App
	if err := gorm.G[data.App](s.DB).Create(ctx, &data.App{
		DeviceID: device.ID,
		Iname:    nightAppID,
		Name:     "Night App",
		Enabled:  true,
		Pushed:   true,
		Order:    2,
	}); err != nil {
		t.Fatalf("failed to create night app: %v", err)
	}

	// Reload device with apps
	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	// 1. Verify Night Mode wins when active
	app, selectedOverride, nextIndex, gapIndex, err := s.determineNextApp(ctx, &d, &user, nil)
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if app == nil || app.Iname != nightAppID {
		t.Errorf("Expected Night Mode app (%s) to take precedence, but got %v", nightAppID, app)
	}

	// 2. Verify Pinned App wins when Night Mode is inactive
	d.NightModeEnabled = false
	app, selectedOverride, nextIndex, gapIndex, err = s.determineNextApp(ctx, &d, &user, nil)
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed (night mode disabled): %v", err)
	}
	if app == nil || app.Iname != pinnedAppID {
		t.Errorf("Expected Pinned app (%s) to be displayed when night mode is inactive, but got %v", pinnedAppID, app)
	}
}

func TestDetermineNextApp_Pinning(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	// Create a user
	user := data.User{Username: "testuser_pin"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	pinnedAppID := "app-2"

	// Create a device with Pinned App
	device := data.Device{
		ID:           "device_pin",
		Username:     user.Username,
		PinnedApp:    &pinnedAppID,
		LastAppIndex: -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	// Create App 1
	app1 := data.App{
		DeviceID: device.ID,
		Iname:    "app-1",
		Name:     "App 1",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app1); err != nil {
		t.Fatalf("failed to create app 1: %v", err)
	}

	// Create App 2 (Pinned)
	app2 := data.App{
		DeviceID: device.ID,
		Iname:    pinnedAppID,
		Name:     "App 2",
		Enabled:  true,
		Pushed:   true,
		Order:    2,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app2); err != nil {
		t.Fatalf("failed to create app 2: %v", err)
	}

	// Reload device with apps
	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	// 1. Verify Sticky Pinning
	// It should return app-2 multiple times, regardless of "rotation"
	for i := range 5 {
		app, _, nextIndex, _, err := s.determineNextApp(ctx, &d, &user, nil)
		if err != nil {
			t.Fatalf("determineNextApp failed: %v", err)
		}
		if app == nil {
			t.Fatalf("expected an app, got nil")
			return
		}

		if app.Iname != pinnedAppID {
			t.Errorf("Iteration %d: Expected Pinned app (%s), got %s", i, pinnedAppID, app.Iname)
		}

		// Update LastAppIndex just like real server would, to ensure it doesn't break pinning
		d.LastAppIndex = nextIndex
	}

	// 2. Verify Missing Pinned App Clears Pin
	missingAppID := "app-missing"
	d.PinnedApp = &missingAppID
	// Update DB to match memory
	if _, err := gorm.G[data.Device](s.DB).Where("id = ?", d.ID).Update(ctx, "pinned_app", missingAppID); err != nil {
		t.Fatalf("failed to update device: %v", err)
	}

	app, selectedOverride, nextIndex, gapIndex, err := s.determineNextApp(ctx, &d, &user, nil)
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed with missing pin: %v", err)
	}
	if app == nil {
		t.Fatal("expected an app (fallback to rotation), got nil")
		return
	}

	// Verify pin is cleared in memory
	if d.PinnedApp != nil {
		t.Error("Expected d.PinnedApp to be nil after missing app check")
	}

	// Verify pin is cleared in DB
	dbDevice, err := gorm.G[data.Device](s.DB).Where("id = ?", d.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to fetch device from DB: %v", err)
	}
	if dbDevice.PinnedApp != nil {
		t.Error("Expected DB PinnedApp to be nil after missing app check")
	}
}

func TestDetermineNextApp_AutoPin(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "autouser"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create device
	device := data.Device{
		ID:           "autodev",
		Username:     user.Username,
		LastAppIndex: -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	// Create AutoPin App
	app := data.App{
		DeviceID: device.ID,
		Iname:    "autopin-app",
		Name:     "AutoPin App",
		Enabled:  true,
		AutoPin:  true,
		Pushed:   true, // Initially treated as successful render
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app); err != nil {
		t.Fatalf("failed to create autopin app: %v", err)
	}

	// Reload device with apps
	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	// 1. Successful Render -> Auto Pin
	// Pushed=true bypasses rendering and success is assumed.
	nextApp, selectedOverride, nextIndex, gapIndex, err := s.determineNextApp(ctx, &d, &user, nil)
	_ = nextApp
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}

	if d.PinnedApp == nil || *d.PinnedApp != app.Iname {
		t.Errorf("Expected app %s to be auto-pinned, but got %v", app.Iname, d.PinnedApp)
	}

	// Check DB
	dbDevice, err := gorm.G[data.Device](s.DB).Where("id = ?", d.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to fetch device from DB: %v", err)
	}
	if dbDevice.PinnedApp == nil || *dbDevice.PinnedApp != app.Iname {
		t.Error("Auto-pin not reflected in DB")
	}

	// 2. Failed Render -> Auto Unpin
	// We must simulate a render failure. determineNextApp calls possiblyRender.
	// possiblyRender returns success=false if EmptyLastRender is true.
	// Note: Pushed=true always returns true in possiblyRender. We need to set Pushed=false and Path!="" to trigger render logic.

	app.Pushed = false
	appPath := "test.star"
	app.Path = &appPath
	app.EmptyLastRender = true
	// LastRender must be old enough to trigger a new render
	app.LastRender = time.Now().Add(-1 * time.Hour)

	updates := data.App{
		Pushed:          false,
		Path:            app.Path,
		EmptyLastRender: true,
		LastRender:      app.LastRender,
	}
	if _, err := gorm.G[data.App](s.DB).Where("id = ?", app.ID).Select("Pushed", "Path", "EmptyLastRender", "LastRender").Updates(ctx, updates); err != nil {
		t.Fatalf("failed to update app: %v", err)
	}

	// Reload d for determineNextApp loop
	d, err = gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device: %v", err)
	}

	// determineNextApp will call possiblyRender for the pinned app.
	// We expect it to fail (EmptyLastRender=true) and then unpin.
	nextApp, selectedOverride, nextIndex, gapIndex, err = s.determineNextApp(ctx, &d, &user, nil)
	_ = nextApp
	_ = selectedOverride
	_ = nextIndex
	_ = gapIndex
	if err != nil {
		t.Fatalf("determineNextApp failed on unpin cycle: %v", err)
	}

	if d.PinnedApp != nil {
		t.Errorf("Expected app to be unpinned after render failure, but still pinned to %v", *d.PinnedApp)
	}

	// Check DB
	dbDevice, err = gorm.G[data.Device](s.DB).Where("id = ?", d.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to fetch device from DB: %v", err)
	}
	if dbDevice.PinnedApp != nil {
		t.Error("Auto-unpin not reflected in DB")
	}
}

func TestDetermineNextApp_InterstitialOverride(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "testuser_interstitial_override"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	device := data.Device{
		ID:           "device_interstitial_override",
		Username:     user.Username,
		LastAppIndex: -1,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	app1 := data.App{
		DeviceID: device.ID,
		Iname:    "app-1",
		Name:     "App 1",
		Enabled:  true,
		Pushed:   true,
		Order:    1,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app1); err != nil {
		t.Fatalf("failed to create app 1: %v", err)
	}

	app2 := data.App{
		DeviceID: device.ID,
		Iname:    "app-2",
		Name:     "App 2",
		Enabled:  true,
		Pushed:   true,
		Order:    2,
	}
	if err := gorm.G[data.App](s.DB).Create(ctx, &app2); err != nil {
		t.Fatalf("failed to create app 2: %v", err)
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	override := data.DeviceOverride{
		ID:             "override-1",
		Kind:           data.OverrideInterstitial,
		DisplayTimeSec: intPtr(7),
	}
	overrides := []data.DeviceOverride{override}

	// First app
	app, ov, nextIndex, _, err := s.determineNextApp(ctx, &d, &user, overrides)
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if ov != nil {
		t.Fatalf("expected regular app first, got override")
	}
	if app == nil || app.Iname != "app-1" {
		t.Fatalf("expected app-1, got %v", app)
	}
	d.LastAppIndex = nextIndex

	// Interstitial override
	app, ov, nextIndex, _, err = s.determineNextApp(ctx, &d, &user, overrides)
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if ov == nil {
		t.Fatalf("expected interstitial override, got nil")
	}
	if app != nil {
		t.Fatalf("expected no app when override selected, got %v", app.Iname)
	}
	servedAt := time.Now()
	ov.LastServedAt = &servedAt
	d.LastSeen = &servedAt
	d.LastAppIndex = nextIndex

	// Next app
	app, ov, _, _, err = s.determineNextApp(ctx, &d, &user, overrides)
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if ov != nil {
		t.Fatalf("expected regular app, got override")
	}
	if app == nil || app.Iname != "app-2" {
		t.Fatalf("expected app-2, got %v", app)
	}
}

func TestDetermineNextApp_InterstitialOverrideFallbacksToApp(t *testing.T) {
	s := newTestServer(t)
	ctx := context.Background()

	user := data.User{Username: "testuser_interstitial_fallback"}
	if err := gorm.G[data.User](s.DB).Create(ctx, &user); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	interstitialIname := "app-int"
	device := data.Device{
		ID:                  "device_interstitial_fallback",
		Username:            user.Username,
		InterstitialEnabled: true,
		InterstitialApp:     &interstitialIname,
		LastAppIndex:        2,
	}
	if err := gorm.G[data.Device](s.DB).Create(ctx, &device); err != nil {
		t.Fatalf("failed to create device: %v", err)
	}

	apps := []data.App{
		{DeviceID: device.ID, Iname: "app-a", Name: "App A", Enabled: true, Pushed: true, Order: 1},
		{DeviceID: device.ID, Iname: "app-b", Name: "App B", Enabled: true, Pushed: true, Order: 2},
		{DeviceID: device.ID, Iname: "app-c", Name: "App C", Enabled: true, Pushed: true, Order: 3},
		{DeviceID: device.ID, Iname: interstitialIname, Name: "Interstitial", Enabled: true, Pushed: true, Order: 4},
	}
	for i := range apps {
		if err := gorm.G[data.App](s.DB).Create(ctx, &apps[i]); err != nil {
			t.Fatalf("failed to create app: %v", err)
		}
	}

	d, err := gorm.G[data.Device](s.DB).Preload("Apps", nil).Where("id = ?", device.ID).First(ctx)
	if err != nil {
		t.Fatalf("failed to reload device with apps: %v", err)
	}

	overrides := []data.DeviceOverride{
		{
			Kind:          data.OverrideInterstitial,
			EveryN:        intPtr(2),
			LastServedGap: intPtr(0),
		},
	}

	app, ov, _, _, err := s.determineNextApp(ctx, &d, &user, overrides)
	if err != nil {
		t.Fatalf("determineNextApp failed: %v", err)
	}
	if ov != nil {
		t.Fatalf("expected interstitial app fallback, got override")
	}
	if app == nil || app.Iname != interstitialIname {
		t.Fatalf("expected interstitial app, got %v", app)
	}
}

func TestShouldServeInterstitialOverride_EveryN(t *testing.T) {
	every2 := 2
	ov := data.DeviceOverride{EveryN: &every2}

	gapsCount := 2 // three apps -> two gaps

	if !shouldServeInterstitialOverride(&ov, 0, gapsCount) {
		t.Fatalf("expected first gap to be eligible when LastServedGap is nil")
	}

	ov.LastServedGap = intPtr(0)
	if shouldServeInterstitialOverride(&ov, 1, gapsCount) {
		t.Fatalf("expected next gap to be skipped when everyN=2")
	}
	if !shouldServeInterstitialOverride(&ov, 0, gapsCount) {
		t.Fatalf("expected wrap-around gap to be eligible when everyN=2")
	}

	every5 := 5
	ov = data.DeviceOverride{EveryN: &every5, LastServedGap: intPtr(0)}
	if shouldServeInterstitialOverride(&ov, 1, gapsCount) {
		t.Fatalf("expected clamped everyN to skip next gap")
	}
}
