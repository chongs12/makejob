// Package repository 提供数据访问层实现
package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// ==================== 用户管理 ====================

// UserStats 用户统计信息
type UserStats struct {
	TotalUsers       int64 `json:"total_users"`
	ProMembers       int64 `json:"pro_members"`
	NewUsersToday    int64 `json:"new_users_today"`
	TodayActiveUsers int64 `json:"today_active_users"`
}

// AdminUserRepository 用户管理仓库接口
type AdminUserRepository interface {
	List(ctx context.Context, page, pageSize int, keyword, role string) ([]model.User, int64, error)
	UpdateRole(ctx context.Context, userID uint, role string) error
	Disable(ctx context.Context, userID uint) error
	Enable(ctx context.Context, userID uint) error
	GetStats(ctx context.Context) (*UserStats, error)
}

// adminUserRepository 用户管理仓库实现
type adminUserRepository struct {
	db *gorm.DB
}

// NewAdminUserRepository 创建用户管理仓库实例
func NewAdminUserRepository(db *gorm.DB) AdminUserRepository {
	return &adminUserRepository{db: db}
}

// List 获取用户列表
func (r *adminUserRepository) List(ctx context.Context, page, pageSize int, keyword, role string) ([]model.User, int64, error) {
	var users []model.User
	var total int64

	query := r.db.WithContext(ctx).Model(&model.User{})

	// 关键词搜索
	if keyword != "" {
		query = query.Where("username LIKE ? OR email LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 角色过滤
	if role != "" {
		query = query.Where("role = ?", role)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计用户数量失败: %w", err)
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}

	return users, total, nil
}

// UpdateRole 更新用户角色
func (r *adminUserRepository) UpdateRole(ctx context.Context, userID uint, role string) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", role)
	if result.Error != nil {
		return fmt.Errorf("更新用户角色失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// Disable 禁用用户
func (r *adminUserRepository) Disable(ctx context.Context, userID uint) error {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("鐢ㄦ埛涓嶅瓨鍦?")
		}
		return fmt.Errorf("鏌ヨ鐢ㄦ埛澶辫触: %w", err)
	}

	nextRole := "disabled"
	if user.Role == "disabled" {
		nextRole = model.UserRoleFreeMember
	}

	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", nextRole)
	if result.Error != nil {
		return fmt.Errorf("禁用用户失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// Enable 启用用户
func (r *adminUserRepository) Enable(ctx context.Context, userID uint) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", model.UserRoleFreeMember)
	if result.Error != nil {
		return fmt.Errorf("启用用户失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("用户不存在")
	}
	return nil
}

// GetStats 获取用户统计信息
func (r *adminUserRepository) GetStats(ctx context.Context) (*UserStats, error) {
	var stats UserStats

	// 总用户数
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, fmt.Errorf("统计总用户数失败: %w", err)
	}

	// Pro会员数
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("membership_level = ? AND membership_expire_at > ?", model.MembershipLevelPro, time.Now()).
		Count(&stats.ProMembers).Error; err != nil {
		return nil, fmt.Errorf("统计Pro会员数失败: %w", err)
	}

	// 今日新增用户
	today := time.Now().Format("2006-01-02")
	if err := r.db.WithContext(ctx).Model(&model.User{}).
		Where("DATE(created_at) = ?", today).
		Count(&stats.NewUsersToday).Error; err != nil {
		return nil, fmt.Errorf("统计今日新增用户失败: %w", err)
	}

	// 今日活跃用户（简化处理：今日注册的用户视为活跃用户）
	stats.TodayActiveUsers = stats.NewUsersToday

	return &stats, nil
}

// ==================== 题库管理 ====================

// AdminQuestionRepository 题库管理仓库接口
type AdminQuestionRepository interface {
	Create(ctx context.Context, question *model.Question) error
	Update(ctx context.Context, question *model.Question) error
	Delete(ctx context.Context, id uint) error
	BatchCreate(ctx context.Context, questions []model.Question) error
	Count(ctx context.Context) (int64, error)
}

// adminQuestionRepository 题库管理仓库实现
type adminQuestionRepository struct {
	db *gorm.DB
}

// NewAdminQuestionRepository 创建题库管理仓库实例
func NewAdminQuestionRepository(db *gorm.DB) AdminQuestionRepository {
	return &adminQuestionRepository{db: db}
}

// Create 创建题目
func (r *adminQuestionRepository) Create(ctx context.Context, question *model.Question) error {
	if err := r.db.WithContext(ctx).Create(question).Error; err != nil {
		return fmt.Errorf("创建题目失败: %w", err)
	}
	return nil
}

// Update 更新题目
func (r *adminQuestionRepository) Update(ctx context.Context, question *model.Question) error {
	if err := r.db.WithContext(ctx).Save(question).Error; err != nil {
		return fmt.Errorf("更新题目失败: %w", err)
	}
	return nil
}

// Delete 删除题目
func (r *adminQuestionRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Question{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除题目失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("题目不存在")
	}
	return nil
}

// BatchCreate 批量创建题目
func (r *adminQuestionRepository) BatchCreate(ctx context.Context, questions []model.Question) error {
	if len(questions) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(questions, 100).Error; err != nil {
		return fmt.Errorf("批量创建题目失败: %w", err)
	}
	return nil
}

// Count 统计题目总数
func (r *adminQuestionRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.Question{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计题目数量失败: %w", err)
	}
	return count, nil
}

// ==================== 行业管理 ====================

// IndustryRepository 行业管理仓库接口
type IndustryRepository interface {
	List(ctx context.Context) ([]model.Industry, error)
	GetByID(ctx context.Context, id uint) (*model.Industry, error)
	Create(ctx context.Context, industry *model.Industry) error
	Update(ctx context.Context, industry *model.Industry) error
	GetByCode(ctx context.Context, code string) (*model.Industry, error)
}

// industryRepository 行业管理仓库实现
type industryRepository struct {
	db *gorm.DB
}

// NewIndustryRepository 创建行业管理仓库实例
func NewIndustryRepository(db *gorm.DB) IndustryRepository {
	return &industryRepository{db: db}
}

// List 获取行业列表
func (r *industryRepository) List(ctx context.Context) ([]model.Industry, error) {
	var industries []model.Industry
	if err := r.db.WithContext(ctx).Order("sort_order ASC, created_at DESC").Find(&industries).Error; err != nil {
		return nil, fmt.Errorf("查询行业列表失败: %w", err)
	}
	return industries, nil
}

// GetByID 根据ID获取行业
func (r *industryRepository) GetByID(ctx context.Context, id uint) (*model.Industry, error) {
	var industry model.Industry
	if err := r.db.WithContext(ctx).First(&industry, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询行业失败: %w", err)
	}
	return &industry, nil
}

// Create 创建行业
func (r *industryRepository) Create(ctx context.Context, industry *model.Industry) error {
	if err := r.db.WithContext(ctx).Create(industry).Error; err != nil {
		return fmt.Errorf("创建行业失败: %w", err)
	}
	return nil
}

// Update 更新行业
func (r *industryRepository) Update(ctx context.Context, industry *model.Industry) error {
	if err := r.db.WithContext(ctx).Save(industry).Error; err != nil {
		return fmt.Errorf("更新行业失败: %w", err)
	}
	return nil
}

// GetByCode 根据代码获取行业
func (r *industryRepository) GetByCode(ctx context.Context, code string) (*model.Industry, error) {
	var industry model.Industry
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&industry).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("根据代码查询行业失败: %w", err)
	}
	return &industry, nil
}

// ==================== 分类管理 ====================

// AdminCategoryRepository 分类管理仓库接口
type AdminCategoryRepository interface {
	List(ctx context.Context) ([]model.Category, error)
	GetByID(ctx context.Context, id uint) (*model.Category, error)
	Create(ctx context.Context, category *model.Category) error
	Update(ctx context.Context, category *model.Category) error
	Delete(ctx context.Context, id uint) error
}

// adminCategoryRepository 分类管理仓库实现
type adminCategoryRepository struct {
	db *gorm.DB
}

// NewAdminCategoryRepository 创建分类管理仓库实例
func NewAdminCategoryRepository(db *gorm.DB) AdminCategoryRepository {
	return &adminCategoryRepository{db: db}
}

// List 获取分类列表
func (r *adminCategoryRepository) List(ctx context.Context) ([]model.Category, error) {
	var categories []model.Category
	if err := r.db.WithContext(ctx).Order("sort_order ASC, created_at DESC").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("查询分类列表失败: %w", err)
	}
	return categories, nil
}

// GetByID 根据ID获取分类
func (r *adminCategoryRepository) GetByID(ctx context.Context, id uint) (*model.Category, error) {
	var category model.Category
	if err := r.db.WithContext(ctx).First(&category, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询分类失败: %w", err)
	}
	return &category, nil
}

// Create 创建分类
func (r *adminCategoryRepository) Create(ctx context.Context, category *model.Category) error {
	if err := r.db.WithContext(ctx).Create(category).Error; err != nil {
		return fmt.Errorf("创建分类失败: %w", err)
	}
	return nil
}

// Update 更新分类
func (r *adminCategoryRepository) Update(ctx context.Context, category *model.Category) error {
	if err := r.db.WithContext(ctx).Save(category).Error; err != nil {
		return fmt.Errorf("更新分类失败: %w", err)
	}
	return nil
}

// Delete 删除分类
func (r *adminCategoryRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Category{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除分类失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("分类不存在")
	}
	return nil
}

// ==================== Prompt模板管理 ====================

// PromptTemplateRepository Prompt模板管理仓库接口
type PromptTemplateRepository interface {
	List(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error)
	GetByID(ctx context.Context, id uint) (*model.PromptTemplate, error)
	Create(ctx context.Context, tpl *model.PromptTemplate) error
	Update(ctx context.Context, tpl *model.PromptTemplate) error
	Delete(ctx context.Context, id uint) error
}

// promptTemplateRepository Prompt模板管理仓库实现
type promptTemplateRepository struct {
	db *gorm.DB
}

// NewPromptTemplateRepository 创建Prompt模板管理仓库实例
func NewPromptTemplateRepository(db *gorm.DB) PromptTemplateRepository {
	return &promptTemplateRepository{db: db}
}

// List 获取Prompt模板列表
func (r *promptTemplateRepository) List(ctx context.Context, industryID *uint, scene string) ([]model.PromptTemplate, error) {
	var templates []model.PromptTemplate
	query := r.db.WithContext(ctx).Model(&model.PromptTemplate{})

	if industryID != nil {
		query = query.Where("industry_id = ?", *industryID)
	}
	if scene != "" {
		query = query.Where("scene = ?", scene)
	}

	if err := query.Order("created_at DESC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("查询Prompt模板列表失败: %w", err)
	}
	return templates, nil
}

// GetByID 根据ID获取Prompt模板
func (r *promptTemplateRepository) GetByID(ctx context.Context, id uint) (*model.PromptTemplate, error) {
	var tpl model.PromptTemplate
	if err := r.db.WithContext(ctx).First(&tpl, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询Prompt模板失败: %w", err)
	}
	return &tpl, nil
}

// Create 创建Prompt模板
func (r *promptTemplateRepository) Create(ctx context.Context, tpl *model.PromptTemplate) error {
	if err := r.db.WithContext(ctx).Create(tpl).Error; err != nil {
		return fmt.Errorf("创建Prompt模板失败: %w", err)
	}
	return nil
}

// Update 更新Prompt模板
func (r *promptTemplateRepository) Update(ctx context.Context, tpl *model.PromptTemplate) error {
	if err := r.db.WithContext(ctx).Save(tpl).Error; err != nil {
		return fmt.Errorf("更新Prompt模板失败: %w", err)
	}
	return nil
}

// Delete 删除Prompt模板
func (r *promptTemplateRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.PromptTemplate{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除Prompt模板失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("Prompt模板不存在")
	}
	return nil
}

// ==================== 配置管理 ====================

// AdminConfigRepository 配置管理仓库接口
type AdminConfigRepository interface {
	List(ctx context.Context) ([]model.AdminConfig, error)
	GetByKey(ctx context.Context, key string) (*model.AdminConfig, error)
	Upsert(ctx context.Context, config *model.AdminConfig) error
	BatchUpsert(ctx context.Context, configs []model.AdminConfig) error
}

// adminConfigRepository 配置管理仓库实现
type adminConfigRepository struct {
	db *gorm.DB
}

// NewAdminConfigRepository 创建配置管理仓库实例
func NewAdminConfigRepository(db *gorm.DB) AdminConfigRepository {
	return &adminConfigRepository{db: db}
}

// List 获取所有配置
func (r *adminConfigRepository) List(ctx context.Context) ([]model.AdminConfig, error) {
	var configs []model.AdminConfig
	if err := r.db.WithContext(ctx).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("查询配置列表失败: %w", err)
	}
	return configs, nil
}

// GetByKey 根据键获取配置
func (r *adminConfigRepository) GetByKey(ctx context.Context, key string) (*model.AdminConfig, error) {
	var config model.AdminConfig
	if err := r.db.WithContext(ctx).Where("config_key = ?", key).First(&config).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询配置失败: %w", err)
	}
	return &config, nil
}

// Upsert 插入或更新配置
func (r *adminConfigRepository) Upsert(ctx context.Context, config *model.AdminConfig) error {
	var existing model.AdminConfig
	err := r.db.WithContext(ctx).Where("config_key = ?", config.ConfigKey).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 不存在则创建
			if err := r.db.WithContext(ctx).Create(config).Error; err != nil {
				return fmt.Errorf("创建配置失败: %w", err)
			}
			return nil
		}
		return fmt.Errorf("查询配置失败: %w", err)
	}
	// 存在则更新
	existing.ConfigValue = config.ConfigValue
	existing.ConfigType = config.ConfigType
	existing.Description = config.Description
	if err := r.db.WithContext(ctx).Save(&existing).Error; err != nil {
		return fmt.Errorf("更新配置失败: %w", err)
	}
	return nil
}

// BatchUpsert 批量插入或更新配置
func (r *adminConfigRepository) BatchUpsert(ctx context.Context, configs []model.AdminConfig) error {
	for i := range configs {
		if err := r.Upsert(ctx, &configs[i]); err != nil {
			return fmt.Errorf("批量更新配置失败: %w", err)
		}
	}
	return nil
}

// ==================== Live2D模型管理 ====================

// Live2DModelRepository Live2D模型管理仓库接口
type Live2DModelRepository interface {
	List(ctx context.Context) ([]model.Live2DModel, error)
	GetByID(ctx context.Context, id uint) (*model.Live2DModel, error)
	Create(ctx context.Context, m *model.Live2DModel) error
	Update(ctx context.Context, m *model.Live2DModel) error
	Delete(ctx context.Context, id uint) error
}

// live2DModelRepository Live2D模型管理仓库实现
type live2DModelRepository struct {
	db *gorm.DB
}

// NewLive2DModelRepository 创建Live2D模型管理仓库实例
func NewLive2DModelRepository(db *gorm.DB) Live2DModelRepository {
	return &live2DModelRepository{db: db}
}

// List 获取Live2D模型列表
func (r *live2DModelRepository) List(ctx context.Context) ([]model.Live2DModel, error) {
	var models []model.Live2DModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("查询Live2D模型列表失败: %w", err)
	}
	return models, nil
}

// GetByID 根据ID获取Live2D模型
func (r *live2DModelRepository) GetByID(ctx context.Context, id uint) (*model.Live2DModel, error) {
	var m model.Live2DModel
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询Live2D模型失败: %w", err)
	}
	return &m, nil
}

// Create 创建Live2D模型
func (r *live2DModelRepository) Create(ctx context.Context, m *model.Live2DModel) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("创建Live2D模型失败: %w", err)
	}
	return nil
}

// Update 更新Live2D模型
func (r *live2DModelRepository) Update(ctx context.Context, m *model.Live2DModel) error {
	if err := r.db.WithContext(ctx).Save(m).Error; err != nil {
		return fmt.Errorf("更新Live2D模型失败: %w", err)
	}
	return nil
}

// Delete 删除Live2D模型
func (r *live2DModelRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.Live2DModel{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除Live2D模型失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("Live2D模型不存在")
	}
	return nil
}

// ==================== TTS配置管理 ====================

// TTSConfigRepository TTS配置管理仓库接口
type TTSConfigRepository interface {
	List(ctx context.Context) ([]model.TTSConfig, error)
	GetByID(ctx context.Context, id uint) (*model.TTSConfig, error)
	Create(ctx context.Context, cfg *model.TTSConfig) error
	Update(ctx context.Context, cfg *model.TTSConfig) error
	Delete(ctx context.Context, id uint) error
}

// ttsConfigRepository TTS配置管理仓库实现
type ttsConfigRepository struct {
	db *gorm.DB
}

// NewTTSConfigRepository 创建TTS配置管理仓库实例
func NewTTSConfigRepository(db *gorm.DB) TTSConfigRepository {
	return &ttsConfigRepository{db: db}
}

// List 获取TTS配置列表
func (r *ttsConfigRepository) List(ctx context.Context) ([]model.TTSConfig, error) {
	var configs []model.TTSConfig
	if err := r.db.WithContext(ctx).Order("sort_order ASC, created_at DESC").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("查询TTS配置列表失败: %w", err)
	}
	return configs, nil
}

// GetByID 根据ID获取TTS配置
func (r *ttsConfigRepository) GetByID(ctx context.Context, id uint) (*model.TTSConfig, error) {
	var cfg model.TTSConfig
	if err := r.db.WithContext(ctx).First(&cfg, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询TTS配置失败: %w", err)
	}
	return &cfg, nil
}

// Create 创建TTS配置
func (r *ttsConfigRepository) Create(ctx context.Context, cfg *model.TTSConfig) error {
	if err := r.db.WithContext(ctx).Create(cfg).Error; err != nil {
		return fmt.Errorf("创建TTS配置失败: %w", err)
	}
	return nil
}

// Update 更新TTS配置
func (r *ttsConfigRepository) Update(ctx context.Context, cfg *model.TTSConfig) error {
	if err := r.db.WithContext(ctx).Save(cfg).Error; err != nil {
		return fmt.Errorf("更新TTS配置失败: %w", err)
	}
	return nil
}

// Delete 删除TTS配置
func (r *ttsConfigRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&model.TTSConfig{}, id)
	if result.Error != nil {
		return fmt.Errorf("删除TTS配置失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("TTS配置不存在")
	}
	return nil
}

// ==================== 面试记录管理 ====================

// MockInterviewRepository 面试记录仓库接口
type MockInterviewRepository interface {
	Count(ctx context.Context) (int64, error)
}

// mockInterviewRepository 面试记录仓库实现
type mockInterviewRepository struct {
	db *gorm.DB
}

// NewMockInterviewRepository 创建面试记录仓库实例
func NewMockInterviewRepository(db *gorm.DB) MockInterviewRepository {
	return &mockInterviewRepository{db: db}
}

// Count 统计面试记录总数
func (r *mockInterviewRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.MockInterview{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("统计面试记录数量失败: %w", err)
	}
	return count, nil
}
