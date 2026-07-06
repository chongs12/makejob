package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob/app/companion/internal/biz"
	"makejob/app/companion/internal/conf"
)

// NewData 创建数据库连接
func NewData(cfg *conf.Data) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.Source), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	if err := db.AutoMigrate(&biz.CompanionSession{}, &biz.ASRConfig{}); err != nil {
		return nil, fmt.Errorf("failed to migrate companion_sessions: %w", err)
	}

	return db, nil
}
