package handlers

import (
	"encoding/json"
	"net/http"
	"github.com/vicradon/yt-downloader/services"
)

type RetryHandler struct {
	conversionService *services.ConversionService
}

func NewRetryHandler(conversionService *services.ConversionService) *RetryHandler {
	return &RetryHandler{
		conversionService: conversionService,
	}
}

func (h *RetryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

    jobID := r.URL.Path[len("/api/retry/"):]
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	err := h.conversionService.RetryJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "retrying",
		"jobId":  jobID,
	})
}
