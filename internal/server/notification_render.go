package server

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"tronbyt-server/internal/data"
)

//go:embed assets/notification.star
var notificationTemplate string

func (s *Server) ensureNotificationTemplatePath() (string, error) {
	dir := filepath.Join(s.DataDir, "notification")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "notification.star")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	if err := os.WriteFile(path, []byte(notificationTemplate), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Server) renderNotificationImage(ctx context.Context, device *data.Device, title, subtitle, subtitle2, iconB64 string) ([]byte, error) {
	path, err := s.ensureNotificationTemplatePath()
	if err != nil {
		return nil, err
	}
	config := map[string]any{
		"title":     title,
		"subtitle":  subtitle,
		"subtitle2": subtitle2,
	}
	if iconB64 != "" {
		config["icon"] = iconB64
	}

	imgBytes, messages, err := s.RenderApp(ctx, device, nil, path, config)
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
