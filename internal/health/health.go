package health

import "sync/atomic"

// HealthChecker отслеживает состояние службы
type HealthChecker struct {
	criticalDegradation atomic.Bool
}

func New() *HealthChecker {
	return &HealthChecker{}
}

func (h *HealthChecker) SetCriticalDegradation(v bool) {
	h.criticalDegradation.Store(v)
}

// IsAlive
func (h *HealthChecker) IsAlive() bool {
	return true
}

// false при критической деградации
func (h *HealthChecker) IsReady() bool {
	return !h.criticalDegradation.Load()
}
