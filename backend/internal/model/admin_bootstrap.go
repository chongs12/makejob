package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"makejob-backend/internal/config"
	applogger "makejob-backend/pkg/logger"

	"go.uber.org/zap"
)

// EnsureAdminUser 启动时检查管理员是否存在，不存在则自动补齐。
func EnsureAdminUser(db *gorm.DB, cfg *config.AdminBootstrapConfig) error {
	if db == nil || cfg == nil || !cfg.Enabled {
		return nil
	}

	var adminCount int64
	if err := db.Model(&User{}).Where("role = ?", UserRoleAdmin).Count(&adminCount).Error; err != nil {
		return fmt.Errorf("check admin count failed: %w", err)
	}
	if adminCount > 0 {
		applogger.Info("admin bootstrap skipped: admin user already exists",
			zap.Int64("admin_count", adminCount))
		return nil
	}

	email := strings.ToLower(strings.TrimSpace(cfg.Email))
	password := strings.TrimSpace(cfg.Password)
	if email == "" || password == "" {
		return fmt.Errorf("admin bootstrap requires admin_bootstrap.email and admin_bootstrap.password when no admin exists")
	}

	username, err := resolveBootstrapUsername(db, strings.TrimSpace(cfg.Username), email)
	if err != nil {
		return err
	}

	membershipLevel := strings.TrimSpace(cfg.MembershipLevel)
	switch membershipLevel {
	case MembershipLevelFree, MembershipLevelPro:
	default:
		membershipLevel = MembershipLevelPro
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password failed: %w", err)
	}

	var existing User
	err = db.Where("email = ?", email).First(&existing).Error
	switch {
	case err == nil:
		updates := map[string]interface{}{
			"role":             UserRoleAdmin,
			"membership_level": membershipLevel,
			"password_hash":    string(passwordHash),
		}
		// Preserve an existing username unless it is empty.
		if strings.TrimSpace(existing.Username) == "" {
			updates["username"] = username
		}

		if err := db.Model(&User{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("promote bootstrap admin failed: %w", err)
		}

		applogger.Warn("admin user was missing; promoted existing user to admin and reset bootstrap password",
			zap.Uint("user_id", existing.ID),
			zap.String("email", email))
		return nil

	case errors.Is(err, gorm.ErrRecordNotFound):
		admin := User{
			Username:        username,
			Email:           email,
			PasswordHash:    string(passwordHash),
			Role:            UserRoleAdmin,
			MembershipLevel: membershipLevel,
		}

		if membershipLevel == MembershipLevelPro {
			expireAt := time.Now().AddDate(10, 0, 0)
			admin.MembershipExpireAt = &expireAt
		}

		if err := db.Create(&admin).Error; err != nil {
			return fmt.Errorf("create bootstrap admin failed: %w", err)
		}

		applogger.Warn("admin user was missing; created bootstrap admin user",
			zap.Uint("user_id", admin.ID),
			zap.String("email", email),
			zap.String("username", admin.Username))
		return nil

	default:
		return fmt.Errorf("query bootstrap admin email failed: %w", err)
	}
}

func resolveBootstrapUsername(db *gorm.DB, preferred, email string) (string, error) {
	candidates := make([]string, 0, 3)
	if preferred != "" {
		candidates = append(candidates, preferred)
	}

	if localPart := strings.TrimSpace(strings.Split(email, "@")[0]); localPart != "" {
		candidates = append(candidates, localPart)
	}

	candidates = append(candidates, "Admin")

	for _, candidate := range candidates {
		username, err := firstAvailableUsername(db, candidate)
		if err != nil {
			return "", err
		}
		if username != "" {
			return username, nil
		}
	}

	return "", fmt.Errorf("resolve bootstrap admin username failed")
}

func firstAvailableUsername(db *gorm.DB, base string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", nil
	}

	names := []string{base}
	for i := 1; i <= 99; i++ {
		names = append(names, fmt.Sprintf("%s%d", base, i))
	}

	for _, name := range names {
		var count int64
		if err := db.Model(&User{}).Where("username = ?", name).Count(&count).Error; err != nil {
			return "", fmt.Errorf("check bootstrap username failed: %w", err)
		}
		if count == 0 {
			return name, nil
		}
	}

	return "", nil
}
