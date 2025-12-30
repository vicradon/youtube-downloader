package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"github.com/vicradon/yt-downloader/services"
)

type DeleteHandler struct {
	storageService *services.StorageService
}

func NewDeleteHandler(storageService *services.StorageService) *DeleteHandler {
	return &DeleteHandler{
		storageService: storageService,
	}
}

func (h *DeleteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

    filename := r.URL.Path[len("/api/delete/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	err := h.storageService.DeleteFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error deleting file", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
