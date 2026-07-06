// Package api provides the HTTP server, middleware, and request handlers.
package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"subpage/internal/api/handlers"
	"subpage/internal/api/middleware"
	"subpage/internal/config"
	"subpage/internal/pkg/subpage"
)

type Server struct {
	app         *fiber.App
	cfg         *config.Config
	configStore *subpage.ConfigStore
}

func New(cfg *config.Config) *Server {
	panel := subpage.NewPanelClient(cfg.RemnawavePanelURL, cfg.RemnawaveAPIToken)
	configStore := subpage.NewConfigStore(panel, cfg.InternalJWTSecret, cfg.SubpageConfigUUID)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          30 * time.Second,
		IdleTimeout:           120 * time.Second,
		// The app always sits behind a reverse proxy (TRUST_PROXY in the
		// Node original), so c.IP()/c.Secure() resolve from X-Forwarded-*.
		EnableTrustedProxyCheck: true,
		TrustedProxies:          []string{"0.0.0.0/0", "::/0"},
		ProxyHeader:             fiber.HeaderXForwardedFor,
	})
	app.Use(recover.New())
	registerRoutes(app, middleware.New(cfg), handlers.NewSubpage(cfg, panel, configStore), cfg.NoWeb)

	return &Server{
		app:         app,
		cfg:         cfg,
		configStore: configStore,
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.configStore.Bootstrap()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", s.cfg.Port)
		if err := s.app.Listen("0.0.0.0:" + s.cfg.Port); err != nil {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.app.ShutdownWithContext(shutdownCtx)
	case err := <-serverErr:
		return err
	}
}
