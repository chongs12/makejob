package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"makejob/app/question/internal/conf"
)

func NewData(cfg *conf.Data) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.Source), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	return db, nil
}
