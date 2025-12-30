package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vicradon/yt-downloader/database"
	"github.com/vicradon/yt-downloader/models"
	"github.com/vicradon/yt-downloader/utils"
)

type ConversionService struct {
	conversions    map[string]*models.ConversionJob
	mu             sync.RWMutex
	ongoingDir     string
	completedDir   string
	storageService *StorageService
}

func NewConversionService(ongoingDir, completedDir string, storageService *StorageService) *ConversionService {
	return &ConversionService{
		conversions:    make(map[string]*models.ConversionJob),
		ongoingDir:     ongoingDir,
		completedDir:   completedDir,
		storageService: storageService,
	}
}

func (s *ConversionService) LoadFromDatabase() error {
	jobs, err := database.LoadConversions()
	if err != nil {
		return err
	}

	for _, job := range jobs {
		jobCopy := job
		s.mu.Lock()
		s.conversions[job.ID] = &jobCopy
		s.mu.Unlock()

		if job.Status == "failed" && job.DownloadURL != "" {
			log.Printf("Found failed job %s with download URL, can be retried", job.ID)
		}
	}

	return nil
}

func (s *ConversionService) CreateJob(jobID, url, format, downloadURL, videoTitle string) *models.ConversionJob {
	job := &models.ConversionJob{
		ID:          jobID,
		URL:         url,
		Format:      format,
		Status:      "downloading",
		StartTime:   time.Now(),
		DownloadURL: downloadURL,
		VideoTitle:  videoTitle,
	}

	s.mu.Lock()
	s.conversions[jobID] = job
	s.mu.Unlock()

	if err := database.SaveConversion(job); err != nil {
		log.Printf("Failed to save job to database: %v", err)
	}

	return job
}

func (s *ConversionService) GetJob(jobID string) (*models.ConversionJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, exists := s.conversions[jobID]
	return job, exists
}

func (s *ConversionService) GetAllJobs() []map[string]interface{} {
	// Fetch fresh data from database
	jobs, err := database.LoadConversions()
	if err != nil {
		log.Printf("Error loading conversions from database: %v", err)
		// Fall back to in-memory data if DB fetch fails
		return s.getJobsFromMemory()
	}

	// Update in-memory map with latest DB data
	s.mu.Lock()
	for _, job := range jobs {
		jobCopy := job
		if existing, exists := s.conversions[job.ID]; exists {
			// Preserve the mutex from existing job
			jobCopy.Mu = existing.Mu
		}
		s.conversions[job.ID] = &jobCopy
	}
	s.mu.Unlock()

	// Now return the jobs
	return s.buildJobResponse(jobs)
}

func (s *ConversionService) getJobsFromMemory() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobsList := make([]*models.ConversionJob, 0, len(s.conversions))
	for _, job := range s.conversions {
		jobsList = append(jobsList, job)
	}

	sort.Slice(jobsList, func(i, j int) bool {
		return jobsList[i].StartTime.After(jobsList[j].StartTime)
	})

	jobs := make([]models.ConversionJob, len(jobsList))
	for i, job := range jobsList {
		jobs[i] = *job
	}

	return s.buildJobResponse(jobs)
}

func (s *ConversionService) buildJobResponse(jobs []models.ConversionJob) []map[string]interface{} {
	// Sort by start time (newest first)
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].StartTime.After(jobs[j].StartTime)
	})

	result := make([]map[string]interface{}, 0, len(jobs))
	for _, job := range jobs {
		var filename string
		if job.Filename != nil {
			filename = *job.Filename
		}
		var errorMsg string
		if job.Error != nil {
			errorMsg = *job.Error
		}
		var endTime interface{}
		if job.EndTime != nil {
			endTime = *job.EndTime
		}
		jobMap := map[string]interface{}{
			"id":         job.ID,
			"url":        job.URL,
			"format":     job.Format,
			"status":     job.Status,
			"startTime":  job.StartTime,
			"endTime":    endTime,
			"filename":   filename,
			"error":      errorMsg,
			"progress":   job.Progress,
			"size":       s.storageService.GetFormattedFileSize(filename),
			"canRetry":   job.Status == "failed" && job.DownloadURL != "",
			"videoTitle": job.VideoTitle, // Add this
		}
		result = append(result, jobMap)
	}

	return result
}

func (s *ConversionService) ProcessConversion(job *models.ConversionJob, downloadURL, format, videoTitle string) {
	job.Mu.Lock()
	job.Status = "downloading"
	job.Progress = 0.25
	database.SaveConversion(job)
	job.Mu.Unlock()

	// Use video title for filename (sanitize it)
	sanitizedTitle := sanitizeFilename(videoTitle)
	if sanitizedTitle == "" {
		sanitizedTitle = job.ID
	}

	tempFile := filepath.Join(s.ongoingDir, sanitizedTitle+".mp4")

	// Download with retries
	if err := s.downloadWithRetries(downloadURL, tempFile, job); err != nil {
		s.markJobFailed(job, "Failed to download video: "+err.Error())
		log.Printf("Job %s failed: %v", job.ID, err)
		return
	}

	job.Mu.Lock()
	job.Status = "converting"
	job.Progress = 0.5
	database.SaveConversion(job)
	job.Mu.Unlock()

	outputFile := filepath.Join(s.completedDir, sanitizedTitle+"."+format)

	cmd := utils.BuildFFmpegCommand(tempFile, outputFile, format)

	if err := cmd.Run(); err != nil {
		s.markJobFailed(job, "FFmpeg conversion failed: "+err.Error())
		log.Printf("Job %s failed: %v", job.ID, err)
		os.Remove(tempFile)
		return
	}

	os.Remove(tempFile)

	job.Mu.Lock()
	job.Status = "completed"
	job.Progress = 1.0
	endTime := time.Now()
	job.EndTime = &endTime
	filename := sanitizedTitle + "." + format
	job.Filename = &filename
	database.SaveConversion(job)
	job.Mu.Unlock()

	log.Printf("Job %s: Conversion completed", job.ID)
}

func (s *ConversionService) downloadWithRetries(downloadURL, outputPath string, job *models.ConversionJob) error {
	var resp *http.Response
	var err error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, err = http.Get(downloadURL)
		if err == nil {
			break
		}
		log.Printf("Job %s: Download attempt %d failed: %v", job.ID, attempt+1, err)
		if attempt < maxRetries-1 {
			time.Sleep(time.Duration(5*(attempt+1)) * time.Second)
		}
	}

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

	log.Printf("Job %s: Downloaded %d bytes", job.ID, size)
	return nil
}

func (s *ConversionService) markJobFailed(job *models.ConversionJob, errorMsg string) {
	job.Mu.Lock()
	job.Status = "failed"
	job.Error = &errorMsg
	endTime := time.Now()
	job.EndTime = &endTime
	database.SaveConversion(job)
	job.Mu.Unlock()
}

func (s *ConversionService) RetryJob(jobID string) error {
	job, exists := s.GetJob(jobID)
	if !exists {
		return fmt.Errorf("job not found")
	}

	if job.DownloadURL == "" {
		return fmt.Errorf("cannot retry: no download URL available")
	}

	job.Mu.Lock()
	job.Status = "downloading"
	job.Error = nil
	job.Progress = 0.25
	job.StartTime = time.Now()
	job.EndTime = nil
	database.SaveConversion(job)

	videoTitle := job.VideoTitle
	if videoTitle == "" {
		videoTitle = jobID
	}
	job.Mu.Unlock()

	go s.ProcessConversion(job, job.DownloadURL, job.Format, videoTitle)

	return nil
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
