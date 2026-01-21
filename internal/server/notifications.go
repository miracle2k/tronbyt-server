package server

import (
	"context"
	"math"
	"time"

	"tronbyt-server/internal/data"

	"gorm.io/gorm"
)

func (s *Server) cleanupExpiredNotifications(ctx context.Context, deviceID string) {
	now := time.Now()
	expired, err := gorm.G[data.DeviceNotification](s.DB).
		Where("device_id = ? AND ends_at IS NOT NULL AND ends_at <= ?", deviceID, now).
		Find(ctx)
	if err != nil {
		return
	}
	for i := range expired {
		s.deleteNotification(ctx, &expired[i])
	}
}

func (s *Server) deleteNotification(ctx context.Context, notification *data.DeviceNotification) {
	if notification == nil {
		return
	}
	overrides, err := gorm.G[data.DeviceOverride](s.DB).
		Where("device_id = ? AND managed_by_notif = ?", notification.DeviceID, notification.ID).
		Find(ctx)
	if err == nil {
		for i := range overrides {
			s.deleteOverride(ctx, &overrides[i])
		}
	}
	if _, err := gorm.G[data.DeviceNotification](s.DB).Where("id = ?", notification.ID).Delete(ctx); err != nil {
		return
	}
}

func (s *Server) reconcileNotifications(ctx context.Context, device *data.Device) {
	if device == nil {
		return
	}
	now := time.Now()
	notifications, err := gorm.G[data.DeviceNotification](s.DB).
		Where("device_id = ?", device.ID).
		Where("(ends_at IS NULL OR ends_at > ?)", now).
		Find(ctx)
	if err != nil {
		return
	}
	for i := range notifications {
		s.updateNotificationInterstitial(ctx, device, &notifications[i], now)
	}
}

func (s *Server) updateNotificationInterstitial(ctx context.Context, device *data.Device, notification *data.DeviceNotification, now time.Time) {
	if notification == nil || notification.InterstitialUntil == nil || notification.InterstitialForever {
		return
	}
	interstitialStart := notification.CreatedAt
	if notification.PinUntil != nil && notification.PinUntil.After(interstitialStart) {
		interstitialStart = *notification.PinUntil
	}
	if now.Before(interstitialStart) || now.After(*notification.InterstitialUntil) {
		return
	}

	maxEveryN := 1
	if device != nil && len(device.Apps) > 1 {
		maxEveryN = len(device.Apps) - 1
	}

	desiredEveryN := notificationEveryN(now, interstitialStart, *notification.InterstitialUntil, maxEveryN)

	ov, err := gorm.G[data.DeviceOverride](s.DB).
		Where("device_id = ? AND kind = ? AND managed_by_notif = ?", device.ID, data.OverrideInterstitial, notification.ID).
		First(ctx)
	if err != nil {
		return
	}
	if ov.EveryN != nil && *ov.EveryN == desiredEveryN {
		return
	}
	updates := data.DeviceOverride{
		EveryN: &desiredEveryN,
	}
	_, _ = gorm.G[data.DeviceOverride](s.DB).Where("id = ?", ov.ID).Select("every_n").Updates(ctx, updates)
}

func notificationEveryN(now, start, end time.Time, maxEveryN int) int {
	if maxEveryN <= 1 {
		return 1
	}
	if !end.After(start) {
		return maxEveryN
	}
	progress := now.Sub(start).Seconds() / end.Sub(start).Seconds()
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	steps := maxEveryN - 1
	everyN := 1 + int(math.Floor(progress*float64(steps)))
	if everyN < 1 {
		return 1
	}
	if everyN > maxEveryN {
		return maxEveryN
	}
	return everyN
}
