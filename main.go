package main

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/vicradon/yt-downloader/config"
	"github.com/vicradon/yt-downloader/database"
	"github.com/vicradon/yt-downloader/handlers"
	"github.com/vicradon/yt-downloader/services"
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
	youtubeService := services.NewYouTubeService(
		config.AppConfig.RapidAPIKey,
		config.AppConfig.RapidAPIHost,
	)

	storageService := services.NewStorageService(config.AppConfig.AbsCompletedDir)

	conversionService := services.NewConversionService(
		config.AppConfig.AbsOngoingDir,
		config.AppConfig.AbsCompletedDir,
		storageService,
	)

	directDownloadService := services.NewDirectDownloadService(
		config.AppConfig.AbsOngoingDir,
		config.AppConfig.AbsCompletedDir,
	)

	// Load existing conversions from database
	if err := conversionService.LoadFromDatabase(); err != nil {
		log.Printf("Warning: Failed to load conversions from database: %v", err)
	}

	// Initialize handlers
	indexHandler := handlers.NewIndexHandler(config.AppConfig.ExecDir)
	conversionsPageHandler := handlers.NewConversionsPageHandler(config.AppConfig.ExecDir)
	downloadHandler := handlers.NewDownloadHandler(youtubeService, conversionService, directDownloadService)
	conversionsHandler := handlers.NewConversionsHandler(conversionService)
	fileHandler := handlers.NewFileHandler(storageService)
	deleteHandler := handlers.NewDeleteHandler(storageService)
	retryHandler := handlers.NewRetryHandler(conversionService)
	directDownloadFileHandler := handlers.NewDirectDownloadFileHandler(directDownloadService, config.AppConfig.AbsCompletedDir)

	// Register static files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(config.AppConfig.ExecDir, "static")))))
	
	// Register page routes
	http.Handle("/", indexHandler)
	http.Handle("/conversions", conversionsPageHandler)
	
	// Register API routes with /api prefix
	http.Handle("/api/download", downloadHandler)
	http.Handle("/api/file/", fileHandler)
	http.Handle("/api/conversions", conversionsHandler)
	http.Handle("/api/delete/", deleteHandler)
	http.Handle("/api/retry/", retryHandler)
	http.Handle("/api/direct-download/", directDownloadFileHandler)

	fmt.Println("Server starting on http://0.0.0.0:8080")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}
