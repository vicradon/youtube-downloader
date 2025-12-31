package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/vicradon/yt-downloader/models"
	"github.com/vicradon/yt-downloader/services"
)

type DownloadHandler struct {
	youtubeService         *services.YouTubeService
	conversionService      *services.ConversionService
	directDownloadService  *services.DirectDownloadService
}

func NewDownloadHandler(youtubeService *services.YouTubeService, conversionService *services.ConversionService, directDownloadService *services.DirectDownloadService) *DownloadHandler {
	return &DownloadHandler{
		youtubeService:        youtubeService,
		conversionService:     conversionService,
		directDownloadService: directDownloadService,
	}
}

func (h *DownloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	videoID, err := h.youtubeService.ExtractVideoID(req.URL)
	if err != nil {
		http.Error(w, "Invalid YouTube URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	rapidResp, err := h.youtubeService.GetDownloadURL(videoID)
	if err != nil {
		http.Error(w, "Failed to get download URL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Waiting 20 seconds for file to be ready...")
	h.youtubeService.WaitForFileReady()
	log.Println("File should be ready now")

	// Fetch video title for filename
	videoTitle := rapidResp.Title
	if videoTitle == "" {
		if title, err := h.youtubeService.GetVideoTitle(videoID); err == nil {
			videoTitle = title
		} else {
			log.Printf("Warning: could not fetch video title: %v", err)
			videoTitle = videoID
		}
	}

	if !req.Convert {
		// Sanitize filename and add .mp4 extension
		sanitizedTitle := sanitizeFilename(videoTitle)
		if sanitizedTitle == "" {
			sanitizedTitle = videoID
		}
		filename := sanitizedTitle + ".mp4"

		// Create direct download record
		downloadID := fmt.Sprintf("%s_%d", videoID, time.Now().Unix())
		download := h.directDownloadService.CreateDownload(downloadID, req.URL, filename)

		// Process download in background
		go h.directDownloadService.ProcessDownload(download, rapidResp.File)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "processing",
			"id":     downloadID,
		})
		return
	}

	jobID := fmt.Sprintf("%s_%d", videoID, time.Now().Unix())
	job := h.conversionService.CreateJob(jobID, req.URL, req.Format, rapidResp.File, videoTitle)

	go h.conversionService.ProcessConversion(job, rapidResp.File, req.Format, videoTitle)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "converting",
		"jobId":  jobID,
	})
}

func sanitizeFilename(filename string) string {
	// Remove invalid characters for filenames
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := filename
	for _, char := range invalid {
		result = strings.ReplaceAll(result, char, "")
	}
	// Limit length
	if len(result) > 200 {
		result = result[:200]
	}
	return strings.TrimSpace(result)
}
