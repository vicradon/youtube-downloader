package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vicradon/yt-downloader/config"
	"github.com/vicradon/yt-downloader/database"
	"github.com/vicradon/yt-downloader/models"
	"github.com/vicradon/yt-downloader/services"
	"github.com/vicradon/yt-downloader/utils"
)

var (
	storageService         *services.StorageService
	conversionService      *services.ConversionService
	directDownloadService  *services.DirectDownloadService
	youtubeService         *services.YouTubeService
)

func main() {
	// Load configuration
	if err := config.Load(); err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database
	if err := database.Init(config.AppConfig.DatabaseURL); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// Initialize services
	storageService = services.NewStorageService(config.AppConfig.AbsCompletedDir)
	conversionService = services.NewConversionService(
		config.AppConfig.AbsOngoingDir,
		config.AppConfig.AbsCompletedDir,
		storageService,
	)
	directDownloadService = services.NewDirectDownloadService(
		config.AppConfig.AbsOngoingDir,
		config.AppConfig.AbsCompletedDir,
	)
	youtubeService = services.NewYouTubeService(
		config.AppConfig.RapidAPIKey,
		config.AppConfig.RapidAPIHost,
	)

	// Load existing conversions
	if err := conversionService.LoadFromDatabase(); err != nil {
		log.Printf("Warning: Failed to load conversions: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Vid Downloader CLI ===")
	fmt.Println()

	for {
		fmt.Println("\nCommands:")
		fmt.Println("  1. list - List downloaded MP4 files")
		fmt.Println("  2. convert - Convert an MP4 file")
		fmt.Println("  3. status - Check conversion status")
		fmt.Println("  4. download - Download a YouTube video")
		fmt.Println("  5. quit - Exit")
		fmt.Print("\nEnter command: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		switch input {
		case "1", "list":
			listFiles()
		case "2", "convert":
			convertFile(reader)
		case "3", "status":
			checkStatus()
		case "4", "download":
			downloadVideo(reader)
		case "5", "quit", "exit":
			fmt.Println("Goodbye!")
			return
		default:
			fmt.Println("Unknown command. Try again.")
		}
	}
}

func listFiles() {
	fmt.Println("\n=== Downloaded MP4 Files ===")

	// List files in completed directory
	completedFiles, err := os.ReadDir(config.AppConfig.AbsCompletedDir)
	if err != nil {
		fmt.Printf("Error reading completed directory: %v\n", err)
		return
	}

	// List files in ongoing directory
	ongoingFiles, err := os.ReadDir(config.AppConfig.AbsOngoingDir)
	if err != nil {
		fmt.Printf("Error reading ongoing directory: %v\n", err)
		return
	}

	if len(completedFiles) == 0 && len(ongoingFiles) == 0 {
		fmt.Println("No files found.")
		return
	}

	if len(ongoingFiles) > 0 {
		fmt.Println("\nOngoing downloads:")
		for i, file := range ongoingFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".mp4") {
				info, _ := file.Info()
				size := storageService.FormatFileSize(info.Size())
				fmt.Printf("  %d. %s (%s)\n", i+1, file.Name(), size)
			}
		}
	}

	if len(completedFiles) > 0 {
		fmt.Println("\nCompleted files:")
		for i, file := range completedFiles {
			if !file.IsDir() {
				info, _ := file.Info()
				size := storageService.FormatFileSize(info.Size())
				fmt.Printf("  %d. %s (%s)\n", i+1, file.Name(), size)
			}
		}
	}
}

func convertFile(reader *bufio.Reader) {
	fmt.Println("\n=== Convert MP4 File ===")

	// List available MP4 files
	ongoingFiles, err := os.ReadDir(config.AppConfig.AbsOngoingDir)
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	mp4Files := []string{}
	for _, file := range ongoingFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".mp4") {
			mp4Files = append(mp4Files, file.Name())
		}
	}

	if len(mp4Files) == 0 {
		fmt.Println("No MP4 files available for conversion.")
		return
	}

	fmt.Println("\nAvailable files:")
	for i, file := range mp4Files {
		fmt.Printf("  %d. %s\n", i+1, file)
	}

	fmt.Print("\nSelect file number: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	var fileIndex int
	_, err = fmt.Sscanf(input, "%d", &fileIndex)
	if err != nil || fileIndex < 1 || fileIndex > len(mp4Files) {
		fmt.Println("Invalid selection.")
		return
	}

	selectedFile := mp4Files[fileIndex-1]

	fmt.Println("\nSelect format:")
	fmt.Println("  1. MPG")
	fmt.Println("  2. AVI")
	fmt.Print("\nEnter format number: ")

	formatInput, _ := reader.ReadString('\n')
	formatInput = strings.TrimSpace(formatInput)

	var format string
	switch formatInput {
	case "1":
		format = "mpg"
	case "2":
		format = "avi"
	default:
		fmt.Println("Invalid format selection.")
		return
	}

	// Start conversion
	inputPath := filepath.Join(config.AppConfig.AbsOngoingDir, selectedFile)
	outputFilename := strings.TrimSuffix(selectedFile, ".mp4") + "." + format
	outputPath := filepath.Join(config.AppConfig.AbsCompletedDir, outputFilename)

	fmt.Printf("\nConverting %s to %s...\n", selectedFile, format)

	// Create a simple job for tracking
	jobID := fmt.Sprintf("cli_%d", time.Now().Unix())
	job := &models.ConversionJob{
		ID:        jobID,
		URL:       "cli-conversion",
		Format:    format,
		Status:    "converting",
		StartTime: time.Now(),
	}

	// Save to database
	if err := database.SaveConversion(job); err != nil {
		log.Printf("Warning: Failed to save job: %v", err)
	}

	// Build and run ffmpeg command
	cmd := utils.BuildFFmpegCommand(inputPath, outputPath, format)

	// Show progress indicator
	done := make(chan bool)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Printf("\r%s Converting...", spinner[i%len(spinner)])
				i++
			}
		}
	}()

	err = cmd.Run()
	done <- true
	fmt.Print("\r") // Clear spinner line

	if err != nil {
		fmt.Printf("✗ Conversion failed: %v\n", err)
		job.Status = "failed"
		errMsg := err.Error()
		job.Error = &errMsg
	} else {
		fmt.Printf("✓ Conversion completed: %s\n", outputFilename)
		job.Status = "completed"
		job.Filename = &outputFilename

		// Optionally delete the original MP4
		fmt.Print("\nDelete original MP4 file? (y/n): ")
		deleteInput, _ := reader.ReadString('\n')
		deleteInput = strings.TrimSpace(strings.ToLower(deleteInput))
		if deleteInput == "y" || deleteInput == "yes" {
			if err := os.Remove(inputPath); err != nil {
				fmt.Printf("Warning: Failed to delete original file: %v\n", err)
			} else {
				fmt.Println("Original file deleted.")
			}
		}
	}

	endTime := time.Now()
	job.EndTime = &endTime
	database.SaveConversion(job)
}

func checkStatus() {
	fmt.Println("\n=== Conversion Status ===")

	jobs := conversionService.GetAllJobs()

	if len(jobs) == 0 {
		fmt.Println("No conversions found.")
		return
	}

	for _, job := range jobs {
		status := job["status"].(string)
		format := job["format"].(string)
		startTime := job["startTime"].(time.Time)

		fmt.Printf("\nJob ID: %s\n", job["id"])
		fmt.Printf("Format: %s\n", strings.ToUpper(format))
		fmt.Printf("Status: %s\n", status)
		fmt.Printf("Started: %s\n", startTime.Format("2006-01-02 15:04:05"))

		switch status {
		case "completed":
			{
				if filename, ok := job["filename"].(string); ok && filename != "" {
					size := job["size"].(string)
					fmt.Printf("File: %s (%s)\n", filename, size)
				}
				if endTime := job["endTime"]; endTime != nil {
					fmt.Printf("Completed: %s\n", endTime.(time.Time).Format("2006-01-02 15:04:05"))
				}
			}
		case "failed":
			{
				if errorMsg, ok := job["error"].(string); ok && errorMsg != "" {
					fmt.Printf("Error: %s\n", errorMsg)
				}
			}
		}

	}
}

func downloadVideo(reader *bufio.Reader) {
	fmt.Println("\n=== Download YouTube Video ===")

	fmt.Print("Enter YouTube URL: ")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if url == "" {
		fmt.Println("URL cannot be empty.")
		return
	}

	// Extract video ID
	videoID, err := youtubeService.ExtractVideoID(url)
	if err != nil {
		fmt.Printf("Invalid YouTube URL: %v\n", err)
		return
	}

	fmt.Printf("\nExtracted video ID: %s\n", videoID)

	// Get download URL from RapidAPI
	fmt.Println("Getting download URL...")
	rapidResp, err := youtubeService.GetDownloadURL(videoID)
	if err != nil {
		fmt.Printf("Failed to get download URL: %v\n", err)
		return
	}

	// Wait for file to be ready
	fmt.Println("Waiting for file to be ready (this may take up to 20 seconds)...")
	youtubeService.WaitForFileReady()
	fmt.Println("File should be ready now")

	// Fetch video title
	videoTitle := rapidResp.Title
	if videoTitle == "" {
		fmt.Println("Fetching video title...")
		if title, err := youtubeService.GetVideoTitle(videoID); err == nil {
			videoTitle = title
		} else {
			fmt.Printf("Warning: could not fetch video title: %v\n", err)
			videoTitle = videoID
		}
	}

	fmt.Printf("Video title: %s\n", videoTitle)

	// Sanitize filename
	sanitizedTitle := sanitizeFilename(videoTitle)
	if sanitizedTitle == "" {
		sanitizedTitle = videoID
	}
	filename := sanitizedTitle + ".mp4"

	// Create download record
	downloadID := fmt.Sprintf("%s_%d", videoID, time.Now().Unix())
	download := directDownloadService.CreateDownload(downloadID, url, filename)

	// Process download in background
	fmt.Printf("\nDownloading %s...\n", filename)
	go directDownloadService.ProcessDownload(download, rapidResp.File)

	// Wait for download to complete
	fmt.Println("Waiting for download to complete...")
	for {
		time.Sleep(2 * time.Second)
		download, exists := directDownloadService.GetDownload(downloadID)
		if !exists {
			fmt.Println("Error: Download record not found")
			return
		}

		if download.Status == "completed" {
			fmt.Printf("✓ Download completed: %s\n", filename)
			fmt.Printf("File saved to: %s\n", filepath.Join(config.AppConfig.AbsCompletedDir, filename))
			return
		}

		if download.Status == "failed" {
			errMsg := "Unknown error"
			if download.Error != nil {
				errMsg = *download.Error
			}
			fmt.Printf("✗ Download failed: %s\n", errMsg)
			return
		}
	}
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
