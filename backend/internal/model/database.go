// Package model 提供数据模型定义
package model

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob-backend/internal/config"
	applogger "makejob-backend/pkg/logger"
)

// DB 全局数据库连接实例
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("数据库配置不完整")
	}

	dsn := cfg.DSN()

	// 配置GORM日志级别
	logLevel := logger.Silent
	// 在debug模式下显示SQL日志
	// logLevel = logger.Info

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 获取底层SQL连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接池失败: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// 测试连接
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("数据库Ping失败: %w", err)
	}

	DB = db
	applogger.Info("数据库连接成功")
	return db, nil
}

// AutoMigrate 自动迁移所有数据表
func AutoMigrate(db *gorm.DB) error {
	models := []interface{}{
		&User{},
		&Industry{},
		&Category{},
		&Question{},
		&UserQuestionRecord{},
		&MockInterview{},
		&InterviewMessage{},
		&LearningPlan{},
		&LearningTask{},
		&UserFavorite{},
		&UserNote{},
		&MembershipOrder{},
		&AdminConfig{},
		&PromptTemplate{},
		&Live2DModel{},
		&TTSConfig{},
		&ScraperTask{},
	}

	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("迁移表失败: %w", err)
		}
	}

	applogger.Info("数据库表迁移完成")
	return nil
}

// CloseDB 关闭数据库连接
func CloseDB() error {
	if DB == nil {
		return nil
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
