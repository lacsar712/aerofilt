package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/lacsar712/aerofilt/internal/model"
)

type PlantService interface {
	Status() model.PlantStatus
	Alarms() []model.AlarmEvent
	Telemetry(n int) []model.TelemetryPoint
	RequestBackwash(r model.BackwashRequest) (model.BackwashResult, error)
	TransitionWash(r model.WashTransitionRequest) (model.WashTransitionResult, error)
	EmergencyStop() error
	ClearEmergencyStop() error
}

type Server struct{ app PlantService }

func NewServer(app PlantService) *Server { return &Server{app: app} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}) })
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/alarms", s.handleAlarms)
	mux.HandleFunc("/api/telemetry", s.handleTelemetry)
	mux.HandleFunc("/api/backwash", s.handleBackwash)
	mux.HandleFunc("/api/wash/transition", s.handleWashTransition)
	mux.HandleFunc("/api/estop", s.handleEStop)
	mux.HandleFunc("/api/estop/clear", s.handleClearEStop)
	return mux
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.app.Status())
}
func (s *Server) handleAlarms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.app.Alarms())
}
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.app.Telemetry(64))
}
func (s *Server) handleBackwash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req model.BackwashRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.IssuedAt.IsZero() {
		req.IssuedAt = time.Now().UTC()
	}
	res, err := s.app.RequestBackwash(req)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}
func (s *Server) handleWashTransition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req model.WashTransitionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.At.IsZero() {
		req.At = time.Now().UTC()
	}
	res, err := s.app.TransitionWash(req)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error(), "phase": string(res.Phase)})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
func (s *Server) handleEStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.app.EmergencyStop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "estop"})
}
func (s *Server) handleClearEStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.app.ClearEmergencyStop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
