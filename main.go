package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"

	"frameworks_4/internal/health"
	"frameworks_4/internal/metrics"
	"frameworks_4/internal/state"
)

var (
	store   *state.Store
	met     *metrics.Metrics
	checker *health.HealthChecker
	logger  *slog.Logger
)

func main() {
	store = state.NewStore()
	met = metrics.New()
	checker = health.New()
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		},
	})

	registerRoutes(app)

	port := "3000"
	logger.Info("сервер запущен", "port", port)
	if err := app.Listen(":" + port); err != nil {
		fmt.Fprintf(os.Stderr, "ошибка запуска: %v\n", err)
		os.Exit(1)
	}
}

// routes
func registerRoutes(app *fiber.App) {
	app.Post("/event", handleEvent)
	app.Get("/process/:key", handleGetProcess)
	app.Get("/processes", handleListProcesses)
	app.Get("/healthz/live", handleLive)
	app.Get("/healthz/ready", handleReady)
	app.Get("/metrics", handleMetrics)
	app.Post("/admin/degrade", handleSetDegrade)
}
