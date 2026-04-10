package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type updateProfileRequest struct {
	FirstName  string  `json:"first_name"`
	LastName   string  `json:"last_name"`
	MiddleName *string `json:"middle_name"`
	Phone      *string `json:"phone"`
	KycLevel   int     `json:"kyc_level"`
}

type UserHandler struct {
	userService *service.UserService
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	email := r.URL.Query().Get("email")

	users, total, err := h.userService.ListUsers(r.Context(), limit, offset, email)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
	})
}

func (h *UserHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	userWithWallets, err := h.userService.GetUserWithWallets(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, userWithWallets)
}

func (h *UserHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	profile, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}


func (h *UserHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	profile, err := h.userService.UpdateProfile(r.Context(), userID, req.FirstName, req.LastName, req.MiddleName, req.Phone, req.KycLevel)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}
