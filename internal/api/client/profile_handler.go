package client

import (
	"encoding/json"
	"net/http"

	"github.com/caspianex/exchange-backend/internal/api/middleware"
	"github.com/caspianex/exchange-backend/internal/service"
	"github.com/caspianex/exchange-backend/pkg/validator"
)

type ProfileHandler struct {
	userService *service.UserService
}

func NewProfileHandler(userService *service.UserService) *ProfileHandler {
	return &ProfileHandler{userService: userService}
}

type updateProfileRequest struct {
	FirstName  string  `json:"first_name" validate:"required"`
	LastName   string  `json:"last_name" validate:"required"`
	MiddleName *string `json:"middle_name"`
}

func (h *ProfileHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	profile, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}

func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := validator.Validate(&req); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch current kyc_level so the client cannot change it
	current, err := h.userService.GetProfile(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}

	profile, err := h.userService.UpdateProfile(r.Context(), userID, req.FirstName, req.LastName, req.MiddleName, current.KycLevel)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, profile)
}
