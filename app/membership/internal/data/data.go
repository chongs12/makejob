package data

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"makejob/app/membership/internal/biz"
	"makejob/app/membership/internal/conf"
)

// NewData 创建数据库连接，并自动迁移会员相关表结构以避免模型与表漂移。
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

	// 自动迁移：保证 membership_orders / user_memberships 表结构与模型一致。
	if err := db.AutoMigrate(&biz.MembershipOrder{}, &biz.UserMembership{}); err != nil {
		return nil, fmt.Errorf("auto migrate membership tables failed: %w", err)
	}

	return db, nil
}
