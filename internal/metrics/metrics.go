package metrics

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics содержит счётчики и замеры задержек
type Metrics struct {
	mu sync.RWMutex

	SuccessfulTransitions atomic.Int64
	FailedTransitions     atomic.Int64
	RedeliveryCount       atomic.Int64
	CompensationCount     atomic.Int64

	stepLatencies map[string][]time.Duration
}

func New() *Metrics {
	return &Metrics{
		stepLatencies: make(map[string][]time.Duration),
	}
}

func (m *Metrics) IncSuccess()      { m.SuccessfulTransitions.Add(1) }
func (m *Metrics) IncFailed()       { m.FailedTransitions.Add(1) }
func (m *Metrics) IncRedelivery()   { m.RedeliveryCount.Add(1) }
func (m *Metrics) IncCompensation() { m.CompensationCount.Add(1) }

func (m *Metrics) RecordLatency(step string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stepLatencies[step] = append(m.stepLatencies[step], d)
}

// средняя задержка в миллисекундах
func avgMs(durations []time.Duration) float64 {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return float64(total.Milliseconds()) / float64(len(durations))
}

// метрики в виде string
func (m *Metrics) Snapshot() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("successful_transitions %d\n", m.SuccessfulTransitions.Load()))
	sb.WriteString(fmt.Sprintf("failed_transitions %d\n", m.FailedTransitions.Load()))
	sb.WriteString(fmt.Sprintf("redelivery_count %d\n", m.RedeliveryCount.Load()))
	sb.WriteString(fmt.Sprintf("compensation_count %d\n", m.CompensationCount.Load()))
	for step, latencies := range m.stepLatencies {
		sb.WriteString(fmt.Sprintf("step_latency_avg_ms{step=%q} %.2f\n", step, avgMs(latencies)))
	}
	return sb.String()
}
