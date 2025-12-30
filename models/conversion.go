package models

import (
	"sync"
	"time"
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
	Mu          sync.Mutex `gorm:"-"`
}

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
	Convert bool   `json:"convert"`
}
