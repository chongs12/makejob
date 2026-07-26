package data

import (
	"fmt"
	"log"

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

	// 知识点专项面试无所属行业，industry_id 需允许 NULL。AutoMigrate 不会移除既有列的 NOT NULL
	// 约束（GORM 已知限制），故显式 DROP NOT NULL：幂等，列已可空时为 no-op。外键 fk_mock_interviews_industry
	// 保留，Postgres 外键允许 NULL。设为 best-effort：mock_interviews 可能由单体用其它账号建表，
	// 当前账号若非 owner 无 ALTER 权限，仅告警不阻断启动——此时需手动执行同条 SQL 完成迁移。
	if err := db.Exec("ALTER TABLE mock_interviews ALTER COLUMN industry_id DROP NOT NULL").Error; err != nil {
		log.Printf("[interview-data] warn: drop NOT NULL on mock_interviews.industry_id failed, apply manually if knowledge interviews still fail: %v", err)
	}

	return db, nil
}
