package server

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	"tronbyt-server/internal/data"
	"tronbyt-server/internal/renderer"
)

//go:embed assets/notification.star
var notificationTemplate []byte

const maxNotificationRenderSourceBytes = 16 * 1024

func (s *Server) renderNotificationImage(ctx context.Context, device *data.Device, title, subtitle, subtitle2, iconB64, level string) ([]byte, error) {
	config := map[string]any{
		"title":     title,
		"subtitle":  subtitle,
		"subtitle2": subtitle2,
		"level":     level,
	}
	if iconB64 != "" {
		config["icon"] = iconB64
	}

	return s.renderNotificationSourceWithID(ctx, device, "notification", notificationTemplate, config)
}

func (s *Server) renderNotificationSource(ctx context.Context, device *data.Device, source []byte, config map[string]any) ([]byte, error) {
	if len(source) == 0 {
		return nil, fmt.Errorf("notification render source is empty")
	}
	if len(source) > maxNotificationRenderSourceBytes {
		return nil, fmt.Errorf("notification render source exceeds %d bytes", maxNotificationRenderSourceBytes)
	}
	return s.renderNotificationSourceWithID(ctx, device, "notification-custom", source, config)
}

func (s *Server) renderNotificationSourceWithID(ctx context.Context, device *data.Device, sourceID string, source []byte, config map[string]any) ([]byte, error) {
	var deviceTimezone string
	var locale *string
	supports2x := false
	var appInterval int
	var filters []string

	if device != nil {
		deviceTimezone = device.GetTimezone()
		locale = device.Locale
		supports2x = device.Type.Supports2x()
		appInterval = device.GetEffectiveDwellTime(nil)
		filters = s.getEffectiveFilters(device, nil)
	} else {
		appInterval = 15
	}

	if config == nil {
		config = map[string]any{}
	}

	imgBytes, messages, err := renderer.RenderSource(
		ctx,
		sourceID,
		source,
		config,
		64, 32,
		time.Duration(appInterval)*time.Second,
		30*time.Second,
		true,
		supports2x,
		&deviceTimezone,
		locale,
		filters,
	)
	for _, msg := range messages {
		slog.Debug("Notification render message", "message", msg)
	}
	if err != nil {
		return nil, err
	}
	if len(imgBytes) == 0 {
		return nil, fmt.Errorf("notification render produced empty image")
	}
	return imgBytes, nil
}
