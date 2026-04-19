package main

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"

	"frameworks_4/internal/state"
)

type EventRequest struct {
	ProcessKey     string `json:"process_key"`
	IdempotencyKey string `json:"idempotency_key"`
	CorrelationID  string `json:"correlation_id"`
	Event          string `json:"event"`
	SimulateFail   bool   `json:"simulate_fail,omitempty"`
}

type EventResponse struct {
	CorrelationID string `json:"correlation_id"`
	ProcessKey    string `json:"process_key"`
	Status        string `json:"status"`
	PrevState     string `json:"prev_state,omitempty"`
	NextState     string `json:"next_state,omitempty"`
	Message       string `json:"message,omitempty"`
}

type DegradeRequest struct {
	Enabled bool `json:"enabled"`
}

func handleEvent(c *fiber.Ctx) error {
	var req EventRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(EventResponse{Status: "error", Message: "невалидный JSON: " + err.Error()})
	}

	if req.ProcessKey == "" || req.IdempotencyKey == "" || req.CorrelationID == "" || req.Event == "" {
		return c.Status(400).JSON(EventResponse{
			CorrelationID: req.CorrelationID,
			Status:        "error",
			Message:       "обязательные поля: process_key, idempotency_key, correlation_id, event",
		})
	}

	cid := req.CorrelationID
	store.GetOrCreate(req.ProcessKey)

	// Проверка идемпотентности
	start := time.Now()
	duplicate, err := store.CheckAndRegisterIdempotency(req.ProcessKey, req.IdempotencyKey)
	if err != nil {
		logger.Error("ошибка проверки идемпотентности",
			"correlation_id", cid, "process_key", req.ProcessKey, "error", err)
		return c.Status(500).JSON(EventResponse{CorrelationID: cid, Status: "error", Message: err.Error()})
	}

	if duplicate {
		met.IncRedelivery()
		p := store.Get(req.ProcessKey)
		logger.Info("повторная доставка — игнорирование",
			"correlation_id", cid,
			"process_key", req.ProcessKey,
			"idempotency_key", req.IdempotencyKey,
			"current_state", string(p.State),
		)
		return c.Status(200).JSON(EventResponse{
			CorrelationID: cid,
			ProcessKey:    req.ProcessKey,
			Status:        "duplicate",
			NextState:     string(p.State),
			Message:       "повторная доставка проигнорирована",
		})
	}

	result, err := store.ApplyEvent(req.ProcessKey, state.Event(req.Event), req.SimulateFail)
	elapsed := time.Since(start)

	if err != nil {
		if result != nil && result.Compensated {
			met.IncFailed()
			met.IncCompensation()
			met.RecordLatency(req.Event, elapsed)

			logger.Warn("сбой шага с компенсацией",
				"correlation_id", cid,
				"process_key", req.ProcessKey,
				"event", req.Event,
				"prev_state", string(result.PrevState),
				"next_state", string(result.NextState),
				"error", err.Error(),
			)
			logger.Info("журнал компенсации",
				"correlation_id", cid,
				"process_key", req.ProcessKey,
				"compensation", "отмена бронирования",
				"new_state", string(result.NextState),
			)

			return c.Status(200).JSON(EventResponse{
				CorrelationID: cid,
				ProcessKey:    req.ProcessKey,
				Status:        "compensated",
				PrevState:     string(result.PrevState),
				NextState:     string(result.NextState),
				Message:       err.Error(),
			})
		}

		met.IncFailed()
		store.SetError(req.ProcessKey)
		logger.Error("сбой перехода",
			"correlation_id", cid,
			"process_key", req.ProcessKey,
			"event", req.Event,
			"error", err.Error(),
		)

		return c.Status(422).JSON(EventResponse{
			CorrelationID: cid,
			ProcessKey:    req.ProcessKey,
			Status:        "error",
			Message:       err.Error(),
		})
	}

	// Успешный переход
	met.IncSuccess()
	met.RecordLatency(req.Event, elapsed)
	logger.Info("журнал перехода",
		"correlation_id", cid,
		"process_key", req.ProcessKey,
		"event", req.Event,
		"prev_state", string(result.PrevState),
		"next_state", string(result.NextState),
		"elapsed_ms", elapsed.Milliseconds(),
	)

	return c.Status(200).JSON(EventResponse{
		CorrelationID: cid,
		ProcessKey:    req.ProcessKey,
		Status:        "ok",
		PrevState:     string(result.PrevState),
		NextState:     string(result.NextState),
	})
}

// GET /process/:key
func handleGetProcess(c *fiber.Ctx) error {
	p := store.Get(c.Params("key"))
	if p == nil {
		return c.Status(404).JSON(fiber.Map{"error": "процесс не найден"})
	}
	return c.JSON(fiber.Map{
		"process_key": p.Key,
		"state":       p.State,
		"created_at":  p.CreatedAt,
		"updated_at":  p.UpdatedAt,
	})
}

// GET /processes
func handleListProcesses(c *fiber.Ctx) error {
	type item struct {
		Key       string      `json:"process_key"`
		State     state.State `json:"state"`
		UpdatedAt time.Time   `json:"updated_at"`
	}
	processes := store.List()
	result := make([]item, 0, len(processes))
	for _, p := range processes {
		result = append(result, item{Key: p.Key, State: p.State, UpdatedAt: p.UpdatedAt})
	}
	b, _ := json.Marshal(result)
	c.Set("Content-Type", "application/json")
	return c.Send(b)
}

// GET /healthz/live
func handleLive(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "alive"})
}

// GET /healthz/ready
func handleReady(c *fiber.Ctx) error {
	if checker.IsReady() {
		return c.JSON(fiber.Map{"status": "ready"})
	}
	return c.Status(503).JSON(fiber.Map{"status": "not ready", "reason": "критическая деградация"})
}

// GET /metrics
func handleMetrics(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(met.Snapshot())
}

// POST /admin/degrade
func handleSetDegrade(c *fiber.Ctx) error {
	var req DegradeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "невалидный JSON"})
	}
	checker.SetCriticalDegradation(req.Enabled)
	logger.Warn("изменение флага деградации", "critical_degradation", req.Enabled)
	return c.JSON(fiber.Map{"critical_degradation": req.Enabled})
}
