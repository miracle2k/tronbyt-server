package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"tronbyt-server/internal/data"

	securejoin "github.com/cyphar/filepath-securejoin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const nightModeOverridePriority = 100

func (s *Server) overridesDir(deviceID string) (string, error) {
	deviceWebpDir, err := s.ensureDeviceImageDir(deviceID)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(deviceWebpDir, "overrides")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func (s *Server) overrideImagePath(deviceID, imageKey string) (string, error) {
	dir, err := s.overridesDir(deviceID)
	if err != nil {
		return "", err
	}
	filename := imageKey + ".webp"
	return securejoin.SecureJoin(dir, filename)
}

func (s *Server) saveOverrideImage(deviceID, overrideID string, data []byte) error {
	path, err := s.overrideImagePath(deviceID, overrideID)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (s *Server) readOverrideImage(deviceID, imageKey string) ([]byte, error) {
	path, err := s.overrideImagePath(deviceID, imageKey)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func (s *Server) deleteOverrideFile(deviceID, imageKey string) {
	if imageKey == "" {
		return
	}
	path, err := s.overrideImagePath(deviceID, imageKey)
	if err != nil {
		slog.Warn("Failed to resolve override image path for delete", "error", err)
		return
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Warn("Failed to delete override image file", "path", path, "error", err)
	}
}

func (s *Server) deleteOverride(ctx context.Context, ov *data.DeviceOverride) {
	if ov == nil {
		return
	}
	if _, err := gorm.G[data.DeviceOverride](s.DB).Where("id = ?", ov.ID).Delete(ctx); err != nil {
		slog.Warn("Failed to delete override record", "override_id", ov.ID, "error", err)
		return
	}
	imageKey := ov.ImageKey
	if imageKey == "" {
		imageKey = ov.ID
	}
	s.deleteOverrideFile(ov.DeviceID, imageKey)
}

func (s *Server) cleanupExpiredOverrides(ctx context.Context, deviceID string) {
	now := time.Now()
	expired, err := gorm.G[data.DeviceOverride](s.DB).
		Where("device_id = ? AND (ends_at IS NOT NULL AND ends_at <= ?)", deviceID, now).
		Find(ctx)
	if err != nil {
		slog.Warn("Failed to query expired overrides", "device_id", deviceID, "error", err)
		return
	}
	for i := range expired {
		s.deleteOverride(ctx, &expired[i])
	}
}

func (s *Server) findActiveOverride(ctx context.Context, deviceID string, kind data.OverrideKind, minPriority int) (*data.DeviceOverride, error) {
	now := time.Now()
	q := gorm.G[data.DeviceOverride](s.DB).
		Where("device_id = ? AND kind = ?", deviceID, kind).
		Where("(starts_at IS NULL OR starts_at <= ?)", now).
		Where("(ends_at IS NULL OR ends_at > ?)", now).
		Order("priority DESC, (last_served_at IS NULL) DESC, last_served_at ASC, created_at ASC")

	if minPriority > 0 {
		q = q.Where("priority >= ?", minPriority)
	}

	ov, err := q.First(ctx)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &ov, nil
}

func (s *Server) listActiveOverrides(ctx context.Context, deviceID string, kind data.OverrideKind, minPriority int) ([]data.DeviceOverride, error) {
	now := time.Now()
	q := gorm.G[data.DeviceOverride](s.DB).
		Where("device_id = ? AND kind = ?", deviceID, kind).
		Where("(starts_at IS NULL OR starts_at <= ?)", now).
		Where("(ends_at IS NULL OR ends_at > ?)", now).
		Order("priority DESC, (last_served_at IS NULL) DESC, last_served_at ASC, created_at ASC")

	if minPriority > 0 {
		q = q.Where("priority >= ?", minPriority)
	}

	overrides, err := q.Find(ctx)
	if err != nil {
		return nil, err
	}
	return overrides, nil
}

func (s *Server) markOverrideServed(ctx context.Context, ov *data.DeviceOverride, gapIndex *int, servedAt time.Time) error {
	if ov == nil {
		return nil
	}
	now := servedAt
	if now.IsZero() {
		now = time.Now()
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		current, err := gorm.G[data.DeviceOverride](tx, clause.Locking{Strength: "UPDATE"}).Where("id = ?", ov.ID).First(ctx)
		if err != nil {
			return err
		}

		updates := data.DeviceOverride{
			LastServedAt: &now,
		}
		if gapIndex != nil {
			updates.LastServedGap = gapIndex
			_, err = gorm.G[data.DeviceOverride](tx).Where("id = ?", current.ID).Select("last_served_at", "last_served_gap").Updates(ctx, updates)
			return err
		}
		_, err = gorm.G[data.DeviceOverride](tx).Where("id = ?", current.ID).Select("last_served_at").Updates(ctx, updates)
		return err
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func (s *Server) getOverrideImage(ctx context.Context, deviceID string, kind data.OverrideKind, minPriority int) ([]byte, *data.DeviceOverride, error) {
	for range 3 {
		ov, err := s.findActiveOverride(ctx, deviceID, kind, minPriority)
		if err != nil {
			return nil, nil, err
		}
		if ov == nil {
			return nil, nil, nil
		}

		imageKey := ov.ImageKey
		if imageKey == "" {
			imageKey = ov.ID
		}

		img, err := s.readOverrideImage(deviceID, imageKey)
		if err != nil {
			slog.Warn("Override image missing, removing override", "override_id", ov.ID, "error", err)
			s.deleteOverride(ctx, ov)
			continue
		}

		if err := s.markOverrideServed(ctx, ov, nil, time.Now()); err != nil {
			return nil, nil, err
		}

		return img, ov, nil
	}

	return nil, nil, nil
}

func (s *Server) peekOverride(ctx context.Context, deviceID string, kind data.OverrideKind, minPriority int) (*data.DeviceOverride, error) {
	return s.findActiveOverride(ctx, deviceID, kind, minPriority)
}

func (s *Server) overrideDisplayTime(ov *data.DeviceOverride) int {
	if ov != nil && ov.DisplayTimeSec != nil && *ov.DisplayTimeSec > 0 {
		return *ov.DisplayTimeSec
	}
	// Nil or non-positive display time means "use device default dwell".
	// Note: a separate default dwell for notification-style overrides could be added later.
	return 0
}

func minOverridePriority(device *data.Device) int {
	if device != nil && device.GetNightModeIsActive() {
		return nightModeOverridePriority
	}
	return 0
}
