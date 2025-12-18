package health

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/caspianex/exchange-backend/pkg/worker"
)

type HealthHandler struct {
	rateUpdater *worker.RateUpdater
	startTime   time.Time
}

func NewHealthHandler(rateUpdater *worker.RateUpdater) *HealthHandler {
	return &HealthHandler{
		rateUpdater: rateUpdater,
		startTime:   time.Now(),
	}
}

type HealthResponse struct {
	Status      string                   `json:"status"`
	Timestamp   time.Time                `json:"timestamp"`
	Uptime      string                   `json:"uptime"`
	Workers     WorkersStatus            `json:"workers"`
}

type WorkersStatus struct {
	RateUpdater worker.HealthStatus `json:"rate_updater"`
}

// Health returns basic health check
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HealthDetailed returns detailed health status including workers
func (h *HealthHandler) HealthDetailed(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Uptime:    time.Since(h.startTime).String(),
		Workers: WorkersStatus{
			RateUpdater: h.rateUpdater.Health(),
		},
	}

	// Check if rate updater is healthy
	if err := h.rateUpdater.IsHealthy(); err != nil {
		response.Status = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Ready returns readiness check (for k8s)
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	// Check if critical services are ready
	if err := h.rateUpdater.IsHealthy(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not Ready"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

// Live returns liveness check (for k8s)
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	// Server is alive if we can respond
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Alive"))
}
