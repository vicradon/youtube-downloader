package handlers

import (
	"fmt"
	"net/http"
	"os"
	"github.com/vicradon/yt-downloader/services"
)

type FileHandler struct {
	storageService *services.StorageService
}

func NewFileHandler(storageService *services.StorageService) *FileHandler {
	return &FileHandler{
		storageService: storageService,
	}
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    filename := r.URL.Path[len("/api/file/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	filePath, err := h.storageService.ValidateFilePath(filename)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	http.ServeFile(w, r, filePath)
}
