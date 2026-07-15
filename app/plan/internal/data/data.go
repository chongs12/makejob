package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob/app/plan/internal/biz"
	"makejob/app/plan/internal/conf"
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

	if err := migratePlanSchema(db); err != nil {
		return nil, err
	}

	return db, nil
}

// migratePlanSchema 启动时确保 plan 服务依赖的核心表已存在，避免缺表导致调整计划或反馈能力直接失败。
func migratePlanSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&biz.LearningPlan{},
		&biz.LearningTask{},
		&biz.TaskFeedback{},
		&biz.PlanAdjustment{},
		&biz.PlanAdjustmentPreview{},
	); err != nil {
		return fmt.Errorf("failed to migrate plan schema: %w", err)
	}
	return nil
}
