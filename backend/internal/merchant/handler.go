package merchant

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fystack/b2b-merchant/internal/auth"
	"github.com/fystack/b2b-merchant/internal/order"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	m, err := h.svc.Register(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		if strings.Contains(err.Error(), "required") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	merchants, err := h.svc.List(r.Context())
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if merchants == nil {
		merchants = []*Merchant{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(merchants)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	res, err := h.svc.Login(r.Context(), req)
	if err != nil {
		if strings.Contains(err.Error(), "invalid credentials") {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if strings.Contains(err.Error(), "required") {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	merchantID, ok := r.Context().Value(auth.MerchantIDKey).(string)
	if !ok || merchantID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	m, err := h.svc.GetByID(r.Context(), merchantID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}

func (h *Handler) GetPrivateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// path: /api/merchants/{id}/private-key
	path := strings.TrimPrefix(r.URL.Path, "/api/merchants/")
	id := strings.TrimSuffix(path, "/private-key")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	key, err := h.svc.GetPrivateKey(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"privateKey": key})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	merchantID, ok := r.Context().Value(auth.MerchantIDKey).(string)
	if !ok || merchantID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	stats, err := h.svc.GetStats(r.Context(), merchantID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) GetExchangeRate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rate, err := order.FetchExchangeRate()
	if err != nil {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "exchange rate service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rate": rate,
		"from": "USDC",
		"to":   "NGN",
	})
}

func (h *Handler) UpdatePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	merchantID, ok := r.Context().Value(auth.MerchantIDKey).(string)
	if !ok || merchantID == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req UpdatePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "current_password and new_password are required", http.StatusBadRequest)
		return
	}
	if len(req.NewPassword) < 6 {
		http.Error(w, "new_password must be at least 6 characters", http.StatusBadRequest)
		return
	}
	if err := h.svc.UpdatePassword(r.Context(), merchantID, req.CurrentPassword, req.NewPassword); err != nil {
		if strings.Contains(err.Error(), "invalid credentials") {
			http.Error(w, "current password is incorrect", http.StatusUnauthorized)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// path: /api/merchants/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/merchants/")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	m, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if m == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(m)
}
