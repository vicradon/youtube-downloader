package database

import (
	"github.com/vicradon/yt-downloader/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(dsn string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	return nil
}

func LoadConversions() ([]models.ConversionJob, error) {
	var jobs []models.ConversionJob
	result := DB.Find(&jobs)
	return jobs, result.Error
}

func SaveConversion(job *models.ConversionJob) error {
	return DB.Save(job).Error
}
