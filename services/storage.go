package services

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type StorageService struct {
	CompletedDir string
}

func NewStorageService(completedDir string) *StorageService {
	return &StorageService{
		CompletedDir: completedDir,
	}
}

func (s *StorageService) GetFormattedFileSize(filename string) string {
	if filename == "" {
		return "0 Bytes"
	}

	filePath := filepath.Join(s.CompletedDir, filename)
	info, err := os.Stat(filePath)
	if err != nil {
		return "0 Bytes"
	}

	return s.FormatFileSize(info.Size())
}

func (s *StorageService) FormatFileSize(bytes int64) string {
	if bytes == 0 {
		return "0 Bytes"
	}
	const k = 1024
	sizes := []string{"Bytes", "KB", "MB", "GB"}
	i := int(math.Log(float64(bytes)) / math.Log(float64(k)))
	return fmt.Sprintf("%.1f %s", float64(bytes)/math.Pow(float64(k), float64(i)), sizes[i])
}

func (s *StorageService) ValidateFilePath(filename string) (string, error) {
	filename = filepath.Clean(filename)
	filePath := filepath.Join(s.CompletedDir, filename)

	absCompletedDir, err := filepath.Abs(s.CompletedDir)
	if err != nil {
		return "", fmt.Errorf("error processing directory path")
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("error processing file path")
	}

	absCompletedDirNormalized := strings.TrimSuffix(absCompletedDir, string(filepath.Separator)) + string(filepath.Separator)
	absFilePathNormalized := strings.TrimSuffix(absFilePath, string(filepath.Separator)) + string(filepath.Separator)

	if !strings.HasPrefix(absFilePathNormalized, absCompletedDirNormalized) {
		return "", fmt.Errorf("invalid file path")
	}

	return filePath, nil
}

func (s *StorageService) DeleteFile(filename string) error {
	filePath := filepath.Join(s.CompletedDir, filename)

	if !strings.HasPrefix(filePath, s.CompletedDir) {
		return fmt.Errorf("invalid file path")
	}

	return os.Remove(filePath)
}

func (s *StorageService) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
