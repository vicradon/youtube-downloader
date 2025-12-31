package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vicradon/yt-downloader/database"
	"github.com/vicradon/yt-downloader/models"
)

type DirectDownloadService struct {
	downloads     map[string]*models.DirectDownload
	mu            sync.RWMutex
	tempDir       string
	completedDir  string
}

func NewDirectDownloadService(tempDir, completedDir string) *DirectDownloadService {
	return &DirectDownloadService{
		downloads:    make(map[string]*models.DirectDownload),
		tempDir:      tempDir,
		completedDir: completedDir,
	}
}

func (s *DirectDownloadService) CreateDownload(id, url, filename string) *models.DirectDownload {
	download := &models.DirectDownload{
		ID:           id,
		URL:          url,
		Filename:     filename,
		DownloadTime: time.Now(),
		Status:       "processing",
	}

	s.mu.Lock()
	s.downloads[id] = download
	s.mu.Unlock()

	if err := database.SaveDirectDownload(download); err != nil {
		log.Printf("Failed to save download to database: %v", err)
	}

	return download
}

func (s *DirectDownloadService) GetDownload(id string) (*models.DirectDownload, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	download, exists := s.downloads[id]
	return download, exists
}

func (s *DirectDownloadService) ProcessDownload(download *models.DirectDownload, downloadURL string) {
	// Create temp file path
	tempFile := filepath.Join(s.tempDir, download.Filename)

	// Download the file
	if err := s.downloadFile(downloadURL, tempFile, download); err != nil {
		s.markDownloadFailed(download, "Failed to download video: "+err.Error())
		log.Printf("Download %s failed: %v", download.ID, err)
		return
	}

	// Move to completed directory
	completedFile := filepath.Join(s.completedDir, download.Filename)
	if err := os.Rename(tempFile, completedFile); err != nil {
		s.markDownloadFailed(download, "Failed to move file: "+err.Error())
		log.Printf("Download %s failed to move file: %v", download.ID, err)
		os.Remove(tempFile)
		return
	}

	// Mark as completed
	s.mu.Lock()
	download.Status = "completed"
	download.Error = nil
	download.UpdatedAt = time.Now()
	s.mu.Unlock()

	if err := database.SaveDirectDownload(download); err != nil {
		log.Printf("Failed to update download in database: %v", err)
	}

	log.Printf("Download %s: Completed successfully", download.ID)
}

func (s *DirectDownloadService) downloadFile(downloadURL, outputPath string, download *models.DirectDownload) error {
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status code: %d", resp.StatusCode)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	size, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save video: %w", err)
	}

	log.Printf("Download %s: Downloaded %d bytes", download.ID, size)
	return nil
}

func (s *DirectDownloadService) markDownloadFailed(download *models.DirectDownload, errorMsg string) {
	s.mu.Lock()
	download.Status = "failed"
	download.Error = &errorMsg
	download.UpdatedAt = time.Now()
	s.mu.Unlock()

	if err := database.SaveDirectDownload(download); err != nil {
		log.Printf("Failed to update download in database: %v", err)
	}
}

func (s *DirectDownloadService) DeleteFile(id string) error {
	download, exists := s.GetDownload(id)
	if !exists {
		return fmt.Errorf("download not found")
	}

	filePath := filepath.Join(s.completedDir, download.Filename)
	if err := os.Remove(filePath); err != nil {
		return err
	}

	log.Printf("Download %s: File deleted", download.ID)
	return nil
}
