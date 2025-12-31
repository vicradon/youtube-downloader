package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/vicradon/yt-downloader/services"
)

type DirectDownloadFileHandler struct {
	directDownloadService *services.DirectDownloadService
	completedDir          string
}

func NewDirectDownloadFileHandler(directDownloadService *services.DirectDownloadService, completedDir string) *DirectDownloadFileHandler {
	return &DirectDownloadFileHandler{
		directDownloadService: directDownloadService,
		completedDir:          completedDir,
	}
}

func (h *DirectDownloadFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract download ID from URL path
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid download ID", http.StatusBadRequest)
		return
	}

	downloadID := parts[3]

	// Get download record
	download, exists := h.directDownloadService.GetDownload(downloadID)
	if !exists {
		http.Error(w, "Download not found", http.StatusNotFound)
		return
	}

	if download.Status != "completed" {
		http.Error(w, "Download not ready", http.StatusAccepted)
		return
	}

	// Serve the file
	filePath := filepath.Join(h.completedDir, download.Filename)

	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error accessing file", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", download.Filename))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

	http.ServeFile(w, r, filePath)

	log.Printf("Served file %s for download %s", download.Filename, downloadID)
}
