package api

import (
	"github.com/gofiber/fiber/v2"

	"subpage/internal/api/handlers"
	"subpage/internal/api/middleware"
)

// registerRoutes wires the middleware chain and route table onto app.
func registerRoutes(app *fiber.App, mw *middleware.Middleware, subpageHandler *handlers.Subpage, noWeb bool) {
	app.Use(mw.WithLogger, mw.WithNoRobots, mw.WithProxyCheck, mw.WithAssetsCookieCheck)

	// --- API routes ---
	// Add new endpoints here, one file per resource (see handlers/health.go
	// for the pattern). Keep them thin: parse request -> call service -> respond.
	app.Get("/api/healthz", handlers.HealthCheck)

	// --- Subscription page: app-config (behind the session cookie) and
	// the shortUuid catch-all that serves either the rendered HTML page or
	// a raw subscription payload, plus the built frontend's static assets.
	if noWeb {
		return
	}

	app.Get(handlers.AppConfigRoute, subpageHandler.AppConfigHandler)
	app.Get("/:shortUuid/:clientType?", subpageHandler.SubpageHandler)
	// Any path that isn't a shortUuid segment or a real static asset
	// (bare "/", or deeper/malformed paths) gets the anti-probing
	// socket-close treatment, matching NotFoundExceptionFilter.
	app.Get("/", middleware.KillConnection)
	app.Use(middleware.KillConnection)
}
