package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/domain"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type StaffHandler struct {
	userService *service.UserService
}

func NewStaffHandler(userService *service.UserService) *StaffHandler {
	return &StaffHandler{userService: userService}
}

func (h *StaffHandler) ListStaff(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	email := r.URL.Query().Get("email")

	staff, total, err := h.userService.ListStaff(r.Context(), limit, offset, email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"staff": staff,
		"total": total,
	})
}

func (h *StaffHandler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var input service.CreateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Email == "" || input.FirstName == "" || input.LastName == "" {
		respondError(w, http.StatusBadRequest, "email, first_name, and last_name are required")
		return
	}

	if !domain.UserRole(input.Role).IsStaffRole() || !domain.UserRole(input.Role).IsValidRole() {
		respondError(w, http.StatusBadRequest, "invalid staff role")
		return
	}

	staff, tempPassword, err := h.userService.CreateStaff(r.Context(), callerID, input)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"staff":         staff,
		"temp_password": tempPassword,
	})
}

func (h *StaffHandler) UpdateStaff(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	staffIDStr := chi.URLParam(r, "id")
	staffID, err := strconv.ParseInt(staffIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid staff ID")
		return
	}

	var input service.UpdateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	staff, err := h.userService.UpdateStaff(r.Context(), callerID, staffID, input)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, staff)
}

func (h *StaffHandler) DeactivateStaff(w http.ResponseWriter, r *http.Request) {
	callerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	staffIDStr := chi.URLParam(r, "id")
	staffID, err := strconv.ParseInt(staffIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid staff ID")
		return
	}

	if err := h.userService.DeactivateStaff(r.Context(), callerID, staffID); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "staff member deactivated"})
}
