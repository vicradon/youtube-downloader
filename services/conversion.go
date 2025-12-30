package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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

func (s *ConversionService) CreateJob(jobID, url, format, downloadURL string) *models.ConversionJob {
	job := &models.ConversionJob{
		ID:          jobID,
		URL:         url,
		Format:      format,
		Status:      "downloading",
		StartTime:   time.Now(),
		DownloadURL: downloadURL,
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect all jobs first
	jobsList := make([]*models.ConversionJob, 0, len(s.conversions))
	for _, job := range s.conversions {
		jobsList = append(jobsList, job)
	}

	// Sort by start time (newest first)
	sort.Slice(jobsList, func(i, j int) bool {
		return jobsList[i].StartTime.After(jobsList[j].StartTime)
	})

	// Build response
	jobs := make([]map[string]interface{}, 0, len(jobsList))
	for _, job := range jobsList {
		job.Mu.Lock()
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
			"id":        job.ID,
			"url":       job.URL,
			"format":    job.Format,
			"status":    job.Status,
			"startTime": job.StartTime,
			"endTime":   endTime,
			"filename":  filename,
			"error":     errorMsg,
			"progress":  job.Progress,
			"size":      s.storageService.GetFormattedFileSize(filename),
			"canRetry":  job.Status == "failed" && job.DownloadURL != "",
		}
		job.Mu.Unlock()
		jobs = append(jobs, jobMap)
	}

	return jobs
}

func (s *ConversionService) ProcessConversion(job *models.ConversionJob, downloadURL, format string) {
	job.Mu.Lock()
	job.Status = "downloading"
	job.Progress = 0.25
	database.SaveConversion(job)
	job.Mu.Unlock()

	tempFile := filepath.Join(s.ongoingDir, job.ID+".mp4")

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

	outputFile := filepath.Join(s.completedDir, job.ID+"."+format)

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
	filename := job.ID + "." + format
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
	job.Mu.Unlock()

	go s.ProcessConversion(job, job.DownloadURL, job.Format)

	return nil
}
