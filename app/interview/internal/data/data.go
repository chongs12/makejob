package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob/app/interview/internal/conf"
	"makejob/app/interview/internal/data/model"
)

// NewData 创建数据库连接（由 Wire 调用）
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

	// AutoMigrate 仅增量建表/加列，不修改已存在列，安全。补齐与其它服务一致的迁移行为，
	// 同时为 mock_interviews 增加 current_index 列（实时面试自动结束判定依赖）。
	if err := db.AutoMigrate(&model.MockInterview{}, &model.InterviewMessage{}, &model.InterviewCodingAttempt{}, &model.InterviewReport{}); err != nil {
		return nil, fmt.Errorf("auto migrate interview tables failed: %w", err)
	}

	return db, nil
}
