package config

import (
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

const (
	CompletedDir = "conversions/completed"
	OngoingDir   = "conversions/ongoing"
)

type Config struct {
	RapidAPIKey  string
	RapidAPIHost string
	DatabaseURL  string
	ExecDir      string
	
	AbsCompletedDir string
	AbsOngoingDir   string
}

var AppConfig *Config

func Load() error {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	rapidAPIKey := os.Getenv("RAPIDAPI_KEY")
	rapidAPIHost := os.Getenv("RAPIDAPI_HOST")
	databaseURL := os.Getenv("DATABASE_URL")

	if rapidAPIKey == "" {
		log.Fatal("RAPIDAPI_KEY environment variable is required")
	}
	if rapidAPIHost == "" {
		log.Fatal("RAPIDAPI_HOST environment variable is required")
	}
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	execDir := getExecutableDir()
	absOngoingDir := filepath.Join(execDir, OngoingDir)
	absCompletedDir := filepath.Join(execDir, CompletedDir)

	AppConfig = &Config{
		RapidAPIKey:     rapidAPIKey,
		RapidAPIHost:    rapidAPIHost,
		DatabaseURL:     databaseURL,
		ExecDir:         execDir,
		AbsCompletedDir: absCompletedDir,
		AbsOngoingDir:   absOngoingDir,
	}

	// Create directories
	if err := os.MkdirAll(absOngoingDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(absCompletedDir, 0755); err != nil {
		return err
	}

	return nil
}

func getExecutableDir() string {
	if dir := os.Getenv("EXEC_DIR"); dir != "" {
		return dir
	}
	return "."
}
