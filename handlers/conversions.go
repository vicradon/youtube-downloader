package handlers

import (
	"encoding/json"
	"net/http"
	"github.com/vicradon/yt-downloader/services"
)

type ConversionsHandler struct {
	conversionService *services.ConversionService
}

func NewConversionsHandler(conversionService *services.ConversionService) *ConversionsHandler {
	return &ConversionsHandler{
		conversionService: conversionService,
	}
}

func (h *ConversionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	jobs := h.conversionService.GetAllJobs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}
