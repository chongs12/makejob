package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"makejob/app/learning_archive/internal/conf"
	"makejob/app/learning_archive/internal/data/model"
)

func NewData(cfg *conf.Data) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.Source), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	if err := db.AutoMigrate(&model.LearningArchiveEntry{}); err != nil {
		return nil, fmt.Errorf("failed to migrate learning_archive_entries: %w", err)
	}

	return db, nil
}
