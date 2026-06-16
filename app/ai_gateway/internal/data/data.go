package data

import (
	"github.com/go-kratos/kratos/v2/errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob/app/ai_gateway/internal/biz"
	"makejob/app/ai_gateway/internal/conf"
)

// NewData 创建数据库连接并执行自动迁移
func NewData(cfg *conf.Data) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Database.Source), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.ServiceUnavailable("DATABASE_CONNECTION_FAILED", "数据库连接失败")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	// 自动迁移 AI 配置、Prompt 模板、调用日志三张表
	if err := db.AutoMigrate(&biz.AIConfig{}, &biz.PromptTemplate{}, &biz.AICallLog{}); err != nil {
		// FIX: 替换fmt.Errorf为kratos errors
		return nil, errors.InternalServer("DATABASE_MIGRATE_FAILED", "数据库迁移失败")
	}

	// 插入默认 Prompt 模板种子数据（仅在表为空时执行）
	if err := seedDefaultPrompts(db); err != nil {
		return nil, errors.InternalServer("SEED_PROMPTS_FAILED", "插入默认Prompt模板失败")
	}

	return db, nil
}
