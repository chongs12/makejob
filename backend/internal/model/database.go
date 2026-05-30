// Package model 提供数据模型定义
package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"
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
	connectTimeout := cfg.ConnectTimeoutSeconds()

	// 配置GORM日志级别
	logLevel := logger.Silent
	// 在debug模式下显示SQL日志
	// logLevel = logger.Info

	applogger.Info("开始初始化数据库连接",
		zap.String("host", cfg.Host),
		zap.Int("port", cfg.Port),
		zap.String("dbname", cfg.DBName),
		zap.Int("connect_timeout_seconds", connectTimeout),
	)

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}
	applogger.Info("数据库原生连接已创建")

	// 获取底层SQL连接池
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// 使用显式超时避免数据库半连接状态导致启动阶段长期阻塞。
	pingCtx, cancel := context.WithTimeout(context.Background(), time.Duration(connectTimeout)*time.Second)
	defer cancel()

	// 测试连接
	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("数据库Ping失败: %w", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("初始化GORM失败: %w", err)
	}

	applogger.Info("数据库驱动初始化完成")

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
		&InterviewCodingAttempt{},
		&InterviewCodingEvent{},
		&LearningPlan{},
		&LearningTask{},
		&LearningTaskFeedback{},
		&LearningTaskDiagnosis{},
		&LearningArchiveEntry{},
		&StudyLog{},
		&UserFavorite{},
		&UserNote{},
		&CommunityPost{},
		&CommunityComment{},
		&CommunityPostLike{},
		&MembershipOrder{},
		&AdminConfig{},
		&AIPreset{},
		&AICallLog{},
		&PromptTemplate{},
		&Live2DModel{},
		&TTSConfig{},
		&ScraperTask{},
		&AsyncTask{},
		&RAGDocument{},
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
