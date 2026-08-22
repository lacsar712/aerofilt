// Package api serves HTTP endpoints for the Biofilter filter control desk.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

// FilterController is the application surface the HTTP layer depends on.
type FilterController interface {
	Status() model.FilterStatus
	IngestTemperature(sample model.TempSample) error
	StartCycle(batchID model.BatchID, operator string) error
	BackwashTransition(req model.BackwashTransitionRequest) (model.BackwashTransitionResult, error)
	EmergencyStop() error
	ClearEmergencyStop() error
	ProfileSnapshot() model.ProfileSnapshot
	Alarms() []model.AlarmEvent
	Telemetry(n int) []model.TelemetryPoint
}

// Server wraps HTTP routes for the filter controller.
type Server struct {
	ctrl   FilterController
	mux    *http.ServeMux
	static http.Handler
}

// NewServer constructs API routes; static may be nil.
func NewServer(ctrl FilterController, static http.Handler) *Server {
	s := &Server{ctrl: ctrl, mux: http.NewServeMux(), static: static}
	s.routes()
	return s
}

// Handler returns the root handler.
func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/v1/status", s.handleStatus)
	s.mux.HandleFunc("/v1/temperature", s.handleTemperature)
	s.mux.HandleFunc("/v1/cycle", s.handleCycle)
	s.mux.HandleFunc("/v1/backwash", s.handleBackwash)
	s.mux.HandleFunc("/v1/estop", s.handleEStop)
	s.mux.HandleFunc("/v1/estop/clear", s.handleEStopClear)
	s.mux.HandleFunc("/v1/profile", s.handleProfile)
	s.mux.HandleFunc("/v1/alarms", s.handleAlarms)
	s.mux.HandleFunc("/v1/telemetry", s.handleTelemetry)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	if s.static != nil {
		s.mux.Handle("/", s.static)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.ctrl.Status())
}

func (s *Server) handleTemperature(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var sample model.TempSample
	if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if sample.At.IsZero() {
		sample.At = time.Now().UTC()
	}
	if err := s.ctrl.IngestTemperature(sample); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleCycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var body struct {
		BatchID  model.BatchID `json:"batchId"`
		Operator string        `json:"operator"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.ctrl.StartCycle(body.BatchID, body.Operator); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) handleBackwash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req model.BackwashTransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	res, err := s.ctrl.BackwashTransition(req)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleEStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.ctrl.EmergencyStop(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "estop"})
}

func (s *Server) handleEStopClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := s.ctrl.ClearEmergencyStop(); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.ctrl.ProfileSnapshot())
}

func (s *Server) handleAlarms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.ctrl.Alarms())
}

func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, s.ctrl.Telemetry(100))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
