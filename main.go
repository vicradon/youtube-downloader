package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	completedDir = "conversions/completed"
	ongoingDir   = "conversions/ongoing"
)

type ConversionJob struct {
	ID          string `gorm:"primaryKey"`
	URL         string
	Format      string
	Status      string
	StartTime   time.Time
	EndTime     *time.Time
	Filename    *string
	Error       *string
	Progress    float64
	DownloadURL string
	mu          sync.Mutex
}

var (
	conversions   = make(map[string]*ConversionJob)
	conversionsMu sync.RWMutex
	db            *gorm.DB

	absCompletedDir string
	absOngoingDir   string

	rapidAPIKey string
	rapidAPIHost string
)

type RapidAPIResponse struct {
	Size         int64  `json:"size"`
	Bitrate      int64  `json:"bitrate"`
	Type         string `json:"type"`
	Quality      int    `json:"quality"`
	Mime         string `json:"mime"`
	Comment      string `json:"comment"`
	File         string `json:"file"`
	ReservedFile string `json:"reserved_file"`
}

type DownloadRequest struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Convert bool   `json:"convert"` // true = convert, false = direct download
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	rapidAPIKey = os.Getenv("RAPIDAPI_KEY")
	rapidAPIHost = os.Getenv("RAPIDAPI_HOST")

	if rapidAPIKey == "" {
		log.Fatal("RAPIDAPI_KEY environment variable is required")
	}

	if rapidAPIHost == "" {
		log.Fatal("RAPIDAPI_HOST environment variable is required")
	}

	execDir := getExecutableDir()

	absOngoingDir = filepath.Join(execDir, ongoingDir)
	absCompletedDir = filepath.Join(execDir, completedDir)

	if err := os.MkdirAll(absOngoingDir, 0755); err != nil {
		log.Fatal("Failed to create ongoing directory:", err)
	}
	if err := os.MkdirAll(absCompletedDir, 0755); err != nil {
		log.Fatal("Failed to create completed directory:", err)
	}

	// Initialize database
	if err := initDB(); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Load existing conversions from database
	if err := loadConversionsFromDB(); err != nil {
		log.Printf("Warning: Failed to load conversions from database: %v", err)
	}

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(execDir, "static")))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/download", downloadHandler)
	http.HandleFunc("/file/", fileDownloadHandler)
	http.HandleFunc("/conversions", conversionsHandler)
	http.HandleFunc("/delete/", deleteHandler)
	http.HandleFunc("/retry/", retryHandler)

	fmt.Println("Server starting on http://0.0.0.0:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

func initDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	// Auto-migrate the schema
	return db.AutoMigrate(&ConversionJob{})
}

func loadConversionsFromDB() error {
	var jobs []ConversionJob
	result := db.Find(&jobs)
	if result.Error != nil {
		return result.Error
	}

	for _, job := range jobs {
		jobCopy := job
		conversionsMu.Lock()
		conversions[job.ID] = &jobCopy
		conversionsMu.Unlock()

		// Retry failed jobs that have a download URL
		if job.Status == "failed" && job.DownloadURL != "" {
			log.Printf("Found failed job %s with download URL, can be retried", job.ID)
		}
	}

	return nil
}

func saveConversionToDB(job *ConversionJob) error {
	return db.Save(job).Error
}

func getExecutableDir() string {
	if dir := os.Getenv("EXEC_DIR"); dir != "" {
		return dir
	}
	return "."
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, filepath.Join(getExecutableDir(), "templates", "index.html"))
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	videoID, err := extractVideoID(req.URL)
	if err != nil {
		http.Error(w, "Invalid YouTube URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	rapidAPIURL := fmt.Sprintf("https://%s/download_video/%s?quality=247", rapidAPIHost, videoID)

	httpReq, err := http.NewRequest("GET", rapidAPIURL, nil)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	httpReq.Header.Add("x-rapidapi-key", rapidAPIKey)
	httpReq.Header.Add("x-rapidapi-host", rapidAPIHost)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		http.Error(w, "Failed to call RapidAPI: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	var rapidResp RapidAPIResponse
	if err := json.Unmarshal(body, &rapidResp); err != nil {
		http.Error(w, "Failed to parse API response", http.StatusInternalServerError)
		return
	}

	if rapidResp.File == "" {
		http.Error(w, "No download URL returned from API", http.StatusInternalServerError)
		return
	}

	log.Println("Waiting 20 seconds for file to be ready...")
	time.Sleep(20 * time.Second)
	log.Println("File should be ready now")

	if !req.Convert {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":      "ready",
			"downloadUrl": rapidResp.File,
		})
		return
	}

	jobID := fmt.Sprintf("%s_%d", videoID, time.Now().Unix())

	job := &ConversionJob{
		ID:          jobID,
		URL:         req.URL,
		Format:      req.Format,
		Status:      "downloading",
		StartTime:   time.Now(),
		DownloadURL: rapidResp.File,
	}

	conversionsMu.Lock()
	conversions[jobID] = job
	conversionsMu.Unlock()

	// Save to database
	if err := saveConversionToDB(job); err != nil {
		log.Printf("Failed to save job to database: %v", err)
	}

	go processConversion(job, rapidResp.File, req.Format)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "converting",
		"jobId":  jobID,
	})
}

func processConversion(job *ConversionJob, downloadURL, format string) {
	job.mu.Lock()
	job.Status = "downloading"
	job.Progress = 0.25
	saveConversionToDB(job)
	job.mu.Unlock()

	tempFile := filepath.Join(absOngoingDir, job.ID+".mp4")

	// Download with retries
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
		job.mu.Lock()
		job.Status = "failed"
		errMsg := "Failed to download video: " + err.Error()
		job.Error = &errMsg
		endTime := time.Now()
		job.EndTime = &endTime
		saveConversionToDB(job)
		job.mu.Unlock()
		log.Printf("Job %s failed: %v", job.ID, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		job.mu.Lock()
		job.Status = "failed"
		errMsg := fmt.Sprintf("Download failed with status code: %d", resp.StatusCode)
		job.Error = &errMsg
		endTime := time.Now()
		job.EndTime = &endTime
		saveConversionToDB(job)
		job.mu.Unlock()
		log.Printf("Job %s failed: status code %d", job.ID, resp.StatusCode)
		return
	}

	out, err := os.Create(tempFile)
	if err != nil {
		job.mu.Lock()
		job.Status = "failed"
		errMsg := "Failed to create file: " + err.Error()
		job.Error = &errMsg
		endTime := time.Now()
		job.EndTime = &endTime
		saveConversionToDB(job)
		job.mu.Unlock()
		log.Printf("Job %s failed: %v", job.ID, err)
		return
	}
	defer out.Close()

	size, err := io.Copy(out, resp.Body)
	if err != nil {
		job.mu.Lock()
		job.Status = "failed"
		errMsg := "Failed to save video: " + err.Error()
		job.Error = &errMsg
		endTime := time.Now()
		job.EndTime = &endTime
		saveConversionToDB(job)
		job.mu.Unlock()
		log.Printf("Job %s failed: %v", job.ID, err)
		return
	}

	log.Printf("Job %s: Downloaded %d bytes", job.ID, size)

	job.mu.Lock()
	job.Status = "converting"
	job.Progress = 0.5
	saveConversionToDB(job)
	job.mu.Unlock()

	outputFile := filepath.Join(absCompletedDir, job.ID+"."+format)

	// Build ffmpeg command for the format
	cmd := buildFFmpegCommand(tempFile, outputFile, format)

	if err := cmd.Run(); err != nil {
		job.mu.Lock()
		job.Status = "failed"
		errMsg := "FFmpeg conversion failed: " + err.Error()
		job.Error = &errMsg
		endTime := time.Now()
		job.EndTime = &endTime
		saveConversionToDB(job)
		job.mu.Unlock()
		log.Printf("Job %s failed: %v", job.ID, err)
		os.Remove(tempFile)
		return
	}

	os.Remove(tempFile)

	job.mu.Lock()
	job.Status = "completed"
	job.Progress = 1.0
	endTime := time.Now()
	job.EndTime = &endTime
	filename := job.ID + "." + format
	job.Filename = &filename
	saveConversionToDB(job)
	job.mu.Unlock()

	log.Printf("Job %s: Conversion completed", job.ID)
}

func retryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := r.URL.Path[len("/retry/"):]
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	conversionsMu.Lock()
	job, exists := conversions[jobID]
	if !exists {
		conversionsMu.Unlock()
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	conversionsMu.Unlock()

	if job.DownloadURL == "" {
		http.Error(w, "Cannot retry: no download URL available", http.StatusBadRequest)
		return
	}

	job.mu.Lock()
	job.Status = "downloading"
	job.Error = nil
	job.Progress = 0.25
	job.StartTime = time.Now()
	job.EndTime = nil
	saveConversionToDB(job)
	job.mu.Unlock()

	go processConversion(job, job.DownloadURL, job.Format)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "retrying",
		"jobId":  jobID,
	})
}

func conversionsHandler(w http.ResponseWriter, r *http.Request) {
	conversionsMu.RLock()
	defer conversionsMu.RUnlock()

	jobs := make([]map[string]interface{}, 0, len(conversions))
	for _, job := range conversions {
		job.mu.Lock()
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
			"size":      getFormattedFileSize(filename),
			"canRetry":  job.Status == "failed" && job.DownloadURL != "",
		}
		job.mu.Unlock()
		jobs = append(jobs, jobMap)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func fileDownloadHandler(w http.ResponseWriter, r *http.Request) {
	filename := r.URL.Path[len("/file/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	filename = filepath.Clean(filename)
	filePath := filepath.Join(absCompletedDir, filename)

	// Ensure the completed directory path is absolute for proper comparison
	absCompletedDir, err := filepath.Abs(absCompletedDir)
	if err != nil {
		http.Error(w, "Error processing directory path", http.StatusInternalServerError)
		return
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "Error processing file path", http.StatusInternalServerError)
		return
	}

	// Normalize paths for comparison (handle both with and without trailing separators)
	absCompletedDirNormalized := strings.TrimSuffix(absCompletedDir, string(filepath.Separator)) + string(filepath.Separator)
	absFilePathNormalized := strings.TrimSuffix(absFilePath, string(filepath.Separator)) + string(filepath.Separator)

	if !strings.HasPrefix(absFilePathNormalized, absCompletedDirNormalized) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
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

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Path[len("/delete/"):]
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(absCompletedDir, filename)

	if !strings.HasPrefix(filePath, absCompletedDir) {
		http.Error(w, "Invalid file path", http.StatusBadRequest)
		return
	}

	err := os.Remove(filePath)
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

func extractVideoID(url string) (string, error) {
	if strings.Contains(url, "youtu.be/") {
		parts := strings.Split(url, "youtu.be/")
		if len(parts) > 1 {
			videoID := strings.Split(parts[1], "?")[0]
			if videoID != "" {
				return videoID, nil
			}
		}
	}

	if strings.Contains(url, "youtube.com/watch") {
		parsedURL := strings.Split(url, "?")
		if len(parsedURL) > 1 {
			params := strings.Split(parsedURL[1], "&")
			for _, param := range params {
				if strings.HasPrefix(param, "v=") {
					return strings.TrimPrefix(param, "v="), nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not extract video ID from URL")
}

func getFormattedFileSize(filename string) string {
	if filename == "" {
		return "0 Bytes"
	}

	filePath := filepath.Join(absCompletedDir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		return "0 Bytes"
	}

	return formatFileSize(info.Size())
}

func formatFileSize(bytes int64) string {
	if bytes == 0 {
		return "0 Bytes"
	}
	const k = 1024
	sizes := []string{"Bytes", "KB", "MB", "GB"}
	i := int(math.Log(float64(bytes)) / math.Log(float64(k)))
	return fmt.Sprintf("%.1f %s", float64(bytes)/math.Pow(float64(k), float64(i)), sizes[i])
}

func buildFFmpegCommand(inputFile, outputFile, format string) *exec.Cmd {
	var args []string

	switch format {
	case "avi":
		args = []string{"-y", "-i", inputFile, "-c:v", "mpeg4", "-c:a", "mp3", outputFile}
	case "mpg":
		args = []string{"-y", "-i", inputFile, "-c:v", "mpeg2video", "-q:v", "2", "-c:a", "mp2", "-b:a", "192k", outputFile}
	default:
		args = []string{"-y", "-i", inputFile, "-c:v", "libx264", "-c:a", "aac", outputFile}
	}

	return exec.Command("ffmpeg", args...)
}
