package data

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"makejob/app/admin/internal/biz"
	"makejob/app/admin/internal/data/model"
)

type adminRepo struct {
	db *gorm.DB
}

// NewAdminRepo 创建管理后台仓库实现
func NewAdminRepo(db *gorm.DB) biz.AdminRepo {
	return &adminRepo{db: db}
}

// ==================== 仪表盘 ====================

func (r *adminRepo) GetDashboard(ctx context.Context) (*biz.Dashboard, error) {
	d := &biz.Dashboard{}
	r.db.WithContext(ctx).Model(&model.User{}).Count(&d.TotalUsers)
	r.db.WithContext(ctx).Model(&model.Question{}).Count(&d.TotalQuestions)
	r.db.WithContext(ctx).Model(&model.MockInterview{}).Count(&d.TotalInterviews)
	r.db.WithContext(ctx).Model(&model.User{}).Where("membership_level = ?", "pro").Count(&d.ProMembers)
	today := time.Now().Format("2006-01-02")
	r.db.WithContext(ctx).Model(&model.User{}).Where("DATE(created_at) = ?", today).Count(&d.NewUsersToday)
	d.TodayActiveUsers = 0 // 需要活跃用户统计表，暂时为0
	return d, nil
}

// ==================== 用户管理 ====================

func (r *adminRepo) ListUsers(ctx context.Context, page, pageSize int32) ([]*biz.UserRecord, int64, error) {
	var total int64
	r.db.WithContext(ctx).Model(&model.User{}).Count(&total)

	var users []model.User
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*biz.UserRecord, len(users))
	for i, u := range users {
		result[i] = &biz.UserRecord{
			ID:                 uint64(u.ID),
			Username:           u.Username,
			Email:              u.Email,
			Role:               u.Role,
			Avatar:             u.Avatar,
			MembershipLevel:    u.MembershipLevel,
			MembershipType:     u.MembershipType,
			MembershipExpireAt: u.MembershipExpireAt,
			IsDisabled:         u.IsDisabled,
			CreatedAt:          u.CreatedAt,
		}
	}
	return result, total, nil
}

func (r *adminRepo) UpdateUserRole(ctx context.Context, userID uint64, role string) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", role)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *adminRepo) DisableUser(ctx context.Context, userID uint64) error {
	var user model.User
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		return err
	}
	// 切换禁用状态
	newRole := user.Role
	if user.IsDisabled {
		newRole = "user"
	} else {
		newRole = "disabled"
	}
	return r.db.WithContext(ctx).Model(&user).Updates(map[string]interface{}{
		"is_disabled": !user.IsDisabled,
		"role":        newRole,
	}).Error
}

// ==================== 题库管理 ====================

func (r *adminRepo) AdminListQuestions(ctx context.Context, page, pageSize int32, keyword, difficulty string, categoryID uint64, industryCode string) ([]*biz.QuestionRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.Question{})
	if keyword != "" {
		query = query.Where("title ILIKE ? OR content ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if difficulty != "" {
		query = query.Where("difficulty = ?", difficulty)
	}
	if categoryID > 0 {
		query = query.Where("category_id = ?", categoryID)
	}
	if industryCode != "" {
		// 需要通过 industry 表关联
		var industry model.Industry
		if err := r.db.WithContext(ctx).Where("code = ?", industryCode).First(&industry).Error; err == nil {
			query = query.Where("industry_id = ?", industry.ID)
		}
	}

	var total int64
	query.Count(&total)

	var questions []model.Question
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&questions).Error; err != nil {
		return nil, 0, err
	}

	// 收集 category_id 和 industry_id 用于批量查询名称
	catIDs := make(map[uint]bool)
	indIDs := make(map[uint]bool)
	for _, q := range questions {
		catIDs[q.CategoryID] = true
		indIDs[q.IndustryID] = true
	}

	catNames := make(map[uint]string)
	indNames := make(map[uint]string)
	if len(catIDs) > 0 {
		var cats []model.Category
		ids := make([]uint, 0, len(catIDs))
		for id := range catIDs {
			ids = append(ids, id)
		}
		r.db.WithContext(ctx).Where("id IN ?", ids).Find(&cats)
		for _, c := range cats {
			catNames[c.ID] = c.Name
		}
	}
	if len(indIDs) > 0 {
		var inds []model.Industry
		ids := make([]uint, 0, len(indIDs))
		for id := range indIDs {
			ids = append(ids, id)
		}
		r.db.WithContext(ctx).Where("id IN ?", ids).Find(&inds)
		for _, ind := range inds {
			indNames[ind.ID] = ind.Name
		}
	}

	result := make([]*biz.QuestionRecord, len(questions))
	for i, q := range questions {
		result[i] = &biz.QuestionRecord{
			ID:                 uint64(q.ID),
			CategoryID:         uint64(q.CategoryID),
			IndustryID:         uint64(q.IndustryID),
			Type:               q.Type,
			Difficulty:         q.Difficulty,
			Title:              q.Title,
			Content:            q.Content,
			OptionsJSON:        q.OptionsJSON,
			Answer:             q.Answer,
			Explanation:        q.Explanation,
			SolutionJSON:       q.SolutionJSON,
			JudgeConfigJSON:    q.JudgeConfigJSON,
			AnswerTemplateJSON: q.AnswerTemplateJSON,
			Tags:               q.Tags,
			IsActive:           q.IsActive,
			CreatedAt:          q.CreatedAt,
			UpdatedAt:          q.UpdatedAt,
			CategoryName:       catNames[q.CategoryID],
			IndustryName:       indNames[q.IndustryID],
		}
	}
	return result, total, nil
}

func (r *adminRepo) CreateQuestion(ctx context.Context, q *biz.QuestionRecord) error {
	m := &model.Question{
		CategoryID:         uint(q.CategoryID),
		IndustryID:         uint(q.IndustryID),
		Type:               q.Type,
		Difficulty:         q.Difficulty,
		Title:              q.Title,
		Content:            q.Content,
		OptionsJSON:        q.OptionsJSON,
		Answer:             q.Answer,
		Explanation:        q.Explanation,
		SolutionJSON:       q.SolutionJSON,
		JudgeConfigJSON:    q.JudgeConfigJSON,
		AnswerTemplateJSON: q.AnswerTemplateJSON,
		Tags:               q.Tags,
		IsActive:           q.IsActive,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateQuestion(ctx context.Context, q *biz.QuestionRecord) error {
	m := &model.Question{}
	m.ID = uint(q.ID)
	updates := map[string]interface{}{
		"category_id":          uint(q.CategoryID),
		"industry_id":          uint(q.IndustryID),
		"type":                 q.Type,
		"difficulty":           q.Difficulty,
		"title":                q.Title,
		"content":              q.Content,
		"options_json":         q.OptionsJSON,
		"answer":               q.Answer,
		"explanation":          q.Explanation,
		"solution_json":        q.SolutionJSON,
		"judge_config_json":    q.JudgeConfigJSON,
		"answer_template_json": q.AnswerTemplateJSON,
		"tags":                 q.Tags,
		"is_active":            q.IsActive,
	}
	return r.db.WithContext(ctx).Model(m).Updates(updates).Error
}

func (r *adminRepo) DeleteQuestion(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Question{}, id).Error
}

func (r *adminRepo) BatchCreateQuestions(ctx context.Context, questions []*biz.QuestionRecord) (int, int, []string) {
	success, fail := 0, 0
	var errors []string
	for _, q := range questions {
		if err := r.CreateQuestion(ctx, q); err != nil {
			fail++
			errors = append(errors, err.Error())
		} else {
			success++
		}
	}
	return success, fail, errors
}

func (r *adminRepo) GetQuestionTagTaxonomy(ctx context.Context) ([]*biz.TagTaxonomyGroup, error) {
	var questions []model.Question
	if err := r.db.WithContext(ctx).Where("tags != '' AND tags IS NOT NULL").Find(&questions).Error; err != nil {
		return nil, err
	}
	tagMap := make(map[string]map[string]bool)
	for _, q := range questions {
		if q.Tags == "" {
			continue
		}
		for _, tag := range splitTags(q.Tags) {
			cat := q.Type
			if tagMap[cat] == nil {
				tagMap[cat] = make(map[string]bool)
			}
			tagMap[cat][tag] = true
		}
	}
	groups := make([]*biz.TagTaxonomyGroup, 0, len(tagMap))
	for cat, tags := range tagMap {
		tagList := make([]string, 0, len(tags))
		for t := range tags {
			tagList = append(tagList, t)
		}
		groups = append(groups, &biz.TagTaxonomyGroup{Category: cat, Tags: tagList})
	}
	return groups, nil
}

func splitTags(tags string) []string {
	result := []string{}
	for _, t := range splitAndTrim(tags, ",") {
		if t != "" {
			result = append(result, t)
		}
	}
	return result
}

func splitAndTrim(s, sep string) []string {
	parts := []string{}
	for _, p := range splitString(s, sep) {
		parts = append(parts, trimSpace(p))
	}
	return parts
}

func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	current := ""
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)-1
	for start <= end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end >= start && (s[end] == ' ' || s[end] == '\t') {
		end--
	}
	if start > end {
		return ""
	}
	return s[start : end+1]
}

// ==================== 分类管理 ====================

func (r *adminRepo) ListCategories(ctx context.Context) ([]*biz.CategoryRecord, error) {
	var cats []model.Category
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.CategoryRecord, len(cats))
	for i, c := range cats {
		parentID := uint64(0)
		if c.ParentID != nil {
			parentID = uint64(*c.ParentID)
		}
		result[i] = &biz.CategoryRecord{
			ID:          uint64(c.ID),
			IndustryID:  uint64(c.IndustryID),
			Name:        c.Name,
			ParentID:    parentID,
			SortOrder:   int32(c.SortOrder),
			Icon:        c.Icon,
			Description: c.Description,
			CreatedAt:   c.CreatedAt,
		}
	}
	return result, nil
}

func (r *adminRepo) CreateCategory(ctx context.Context, c *biz.CategoryRecord) error {
	m := &model.Category{
		IndustryID:  uint(c.IndustryID),
		Name:        c.Name,
		SortOrder:   int(c.SortOrder),
		Icon:        c.Icon,
		Description: c.Description,
	}
	if c.ParentID > 0 {
		pid := uint(c.ParentID)
		m.ParentID = &pid
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateCategory(ctx context.Context, c *biz.CategoryRecord) error {
	m := &model.Category{}
	m.ID = uint(c.ID)
	updates := map[string]interface{}{
		"industry_id": uint(c.IndustryID),
		"name":        c.Name,
		"sort_order":  int(c.SortOrder),
		"icon":        c.Icon,
		"description": c.Description,
	}
	if c.ParentID > 0 {
		pid := uint(c.ParentID)
		updates["parent_id"] = &pid
	} else {
		updates["parent_id"] = nil
	}
	return r.db.WithContext(ctx).Model(m).Updates(updates).Error
}

func (r *adminRepo) DeleteCategory(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Category{}, id).Error
}

// ==================== 行业管理 ====================

func (r *adminRepo) ListIndustries(ctx context.Context) ([]*biz.IndustryRecord, error) {
	var inds []model.Industry
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&inds).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.IndustryRecord, len(inds))
	for i, ind := range inds {
		result[i] = &biz.IndustryRecord{
			ID:          uint64(ind.ID),
			Code:        ind.Code,
			Name:        ind.Name,
			Description: ind.Description,
			Icon:        ind.Icon,
			IsActive:    ind.IsActive,
			SortOrder:   int32(ind.SortOrder),
			CreatedAt:   ind.CreatedAt,
		}
	}
	return result, nil
}

func (r *adminRepo) CreateIndustry(ctx context.Context, ind *biz.IndustryRecord) error {
	m := &model.Industry{
		Code:        ind.Code,
		Name:        ind.Name,
		Description: ind.Description,
		Icon:        ind.Icon,
		IsActive:    true,
		SortOrder:   int(ind.SortOrder),
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateIndustry(ctx context.Context, ind *biz.IndustryRecord) error {
	m := &model.Industry{}
	m.ID = uint(ind.ID)
	updates := map[string]interface{}{
		"code":        ind.Code,
		"name":        ind.Name,
		"description": ind.Description,
		"icon":        ind.Icon,
		"is_active":   ind.IsActive,
		"sort_order":  int(ind.SortOrder),
	}
	return r.db.WithContext(ctx).Model(m).Updates(updates).Error
}

func (r *adminRepo) GetIndustryByCode(ctx context.Context, code string) (*biz.IndustryRecord, error) {
	var ind model.Industry
	if err := r.db.WithContext(ctx).Where("code = ?", code).First(&ind).Error; err != nil {
		return nil, err
	}
	return &biz.IndustryRecord{
		ID:          uint64(ind.ID),
		Code:        ind.Code,
		Name:        ind.Name,
		Description: ind.Description,
		Icon:        ind.Icon,
		IsActive:    ind.IsActive,
		SortOrder:   int32(ind.SortOrder),
		CreatedAt:   ind.CreatedAt,
	}, nil
}

// ==================== AI 预设 ====================

func (r *adminRepo) ListAIPresets(ctx context.Context) ([]*biz.AIPreset, error) {
	var models_ []model.AIPreset
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models_).Error; err != nil {
		return nil, err
	}
	presets := make([]*biz.AIPreset, len(models_))
	for i, m := range models_ {
		configs := make(map[string]string)
		if m.ConfigJSON != "" {
			_ = json.Unmarshal([]byte(m.ConfigJSON), &configs)
		}
		presets[i] = &biz.AIPreset{
			ID:        uint64(m.ID),
			Name:      m.Name,
			Configs:   configs,
			IsActive:  m.IsActive,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		}
	}
	return presets, nil
}

func (r *adminRepo) SaveAIPreset(ctx context.Context, preset *biz.AIPreset) error {
	configsJSON, _ := json.Marshal(preset.Configs)
	m := &model.AIPreset{
		Name:       preset.Name,
		ConfigJSON: string(configsJSON),
		IsActive:   preset.IsActive,
	}
	if preset.ID > 0 {
		m.ID = uint(preset.ID)
		return r.db.WithContext(ctx).Save(m).Error
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) CreateAIPreset(ctx context.Context, preset *biz.AIPreset) error {
	configsJSON, _ := json.Marshal(preset.Configs)
	m := &model.AIPreset{
		Name:       preset.Name,
		ConfigJSON: string(configsJSON),
		IsActive:   false,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateAIPreset(ctx context.Context, preset *biz.AIPreset) error {
	configsJSON, _ := json.Marshal(preset.Configs)
	return r.db.WithContext(ctx).Model(&model.AIPreset{}).Where("id = ?", preset.ID).Updates(map[string]interface{}{
		"name":        preset.Name,
		"config_json": string(configsJSON),
	}).Error
}

func (r *adminRepo) DeleteAIPreset(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.AIPreset{}, id).Error
}

func (r *adminRepo) GetAIPresetByID(ctx context.Context, id uint64) (*biz.AIPreset, error) {
	var m model.AIPreset
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	configs := make(map[string]string)
	if m.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(m.ConfigJSON), &configs)
	}
	return &biz.AIPreset{
		ID:       uint64(m.ID),
		Name:     m.Name,
		Configs:  configs,
		IsActive: m.IsActive,
	}, nil
}

func (r *adminRepo) ApplyAIPreset(ctx context.Context, id uint64) error {
	// 先获取预设
	preset, err := r.GetAIPresetByID(ctx, id)
	if err != nil {
		return err
	}
	// 清除所有活跃状态
	r.db.WithContext(ctx).Model(&model.AIPreset{}).Where("1 = 1").Update("is_active", false)
	// 设置当前预设为活跃
	r.db.WithContext(ctx).Model(&model.AIPreset{}).Where("id = ?", id).Update("is_active", true)
	// 将预设配置写入 admin_configs
	return r.BatchUpsertConfigs(ctx, preset.Configs)
}

// ==================== Prompt 模板 ====================

func (r *adminRepo) ListPromptTemplates(ctx context.Context, industryCode string) ([]*biz.PromptTemplate, error) {
	var models_ []model.PromptTemplate
	query := r.db.WithContext(ctx)
	if industryCode != "" {
		var industry model.Industry
		if err := r.db.WithContext(ctx).Where("code = ?", industryCode).First(&industry).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return []*biz.PromptTemplate{}, nil
			}
			return nil, err
		}
		query = query.Where("industry_id = ?", industry.ID)
	}
	if err := query.Order("created_at DESC").Find(&models_).Error; err != nil {
		return nil, err
	}
	templates := make([]*biz.PromptTemplate, len(models_))
	for i, m := range models_ {
		industryID := uint64(0)
		if m.IndustryID != nil {
			industryID = uint64(*m.IndustryID)
		}
		templates[i] = &biz.PromptTemplate{
			ID:              uint64(m.ID),
			IndustryID:      industryID,
			Name:            m.Name,
			IndustryCode:    m.IndustryCode,
			TemplateType:    m.TemplateType,
			Scene:           m.Scene,
			TemplateContent: m.TemplateContent,
			Variables:       m.Variables,
			IsActive:        m.IsActive,
			UpdatedAt:       m.UpdatedAt,
		}
	}
	return templates, nil
}

func (r *adminRepo) SavePromptTemplate(ctx context.Context, tpl *biz.PromptTemplate) error {
	m := &model.PromptTemplate{
		Name:            tpl.Name,
		IndustryCode:    tpl.IndustryCode,
		TemplateType:    tpl.TemplateType,
		TemplateContent: tpl.TemplateContent,
	}
	if tpl.ID > 0 {
		m.ID = uint(tpl.ID)
		return r.db.WithContext(ctx).Save(m).Error
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) CreatePromptTemplate(ctx context.Context, tpl *biz.PromptTemplate) error {
	m := &model.PromptTemplate{
		Name:            tpl.Name,
		Scene:           tpl.Scene,
		TemplateContent: tpl.TemplateContent,
		Variables:       tpl.Variables,
		IsActive:        tpl.IsActive,
	}
	if tpl.IndustryID > 0 {
		industryID := uint(tpl.IndustryID)
		m.IndustryID = &industryID
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdatePromptTemplate(ctx context.Context, tpl *biz.PromptTemplate) error {
	updates := map[string]interface{}{
		"name":             tpl.Name,
		"scene":            tpl.Scene,
		"template_content": tpl.TemplateContent,
		"variables":        tpl.Variables,
		"is_active":        tpl.IsActive,
	}
	if tpl.IndustryID > 0 {
		industryID := uint(tpl.IndustryID)
		updates["industry_id"] = &industryID
	} else {
		updates["industry_id"] = nil
	}
	return r.db.WithContext(ctx).Model(&model.PromptTemplate{}).Where("id = ?", tpl.ID).Updates(updates).Error
}

func (r *adminRepo) DeletePromptTemplate(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.PromptTemplate{}, id).Error
}

// ==================== 系统配置 ====================

func (r *adminRepo) GetAdminConfig(ctx context.Context, key string) (string, error) {
	var cfg model.AdminConfig
	if err := r.db.WithContext(ctx).First(&cfg, "key = ?", key).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return cfg.Value, nil
}

func (r *adminRepo) SetAdminConfig(ctx context.Context, key, value string) error {
	cfg := &model.AdminConfig{Key: key, Value: value}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(cfg).Error
}

func (r *adminRepo) ListAdminConfigs(ctx context.Context) ([]*biz.AdminConfigItem, error) {
	var configs []model.AdminConfig
	if err := r.db.WithContext(ctx).Find(&configs).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.AdminConfigItem, len(configs))
	for i, c := range configs {
		result[i] = &biz.AdminConfigItem{
			Key:         c.Key,
			Value:       c.Value,
			ConfigType:  c.ConfigType,
			Description: c.Description,
		}
	}
	return result, nil
}

func (r *adminRepo) BatchUpsertConfigs(ctx context.Context, configs map[string]string) error {
	for key, value := range configs {
		cfg := &model.AdminConfig{Key: key, Value: value}
		if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(cfg).Error; err != nil {
			return err
		}
	}
	return nil
}

// ==================== AI 调用日志 ====================

// ListAICallLogs 按筛选条件读取 AI 调用日志，保持与单体后台的查询字段一致。
func (r *adminRepo) ListAICallLogs(ctx context.Context, filter biz.AICallLogListFilter) ([]*biz.AICallLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.AICallLog{})
	if filter.AgentType != "" {
		query = query.Where("agent_type = ?", filter.AgentType)
	}
	if filter.Scene != "" {
		query = query.Where("scene = ?", filter.Scene)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.TraceID != "" {
		query = query.Where("trace_id = ?", filter.TraceID)
	}
	if filter.TaskID != nil {
		query = query.Where("task_id = ?", *filter.TaskID)
	}

	var total int64
	query.Count(&total)

	var models_ []model.AICallLog
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(filter.PageSize)).Find(&models_).Error; err != nil {
		return nil, 0, err
	}

	logs := make([]*biz.AICallLog, len(models_))
	for i, m := range models_ {
		logs[i] = &biz.AICallLog{
			ID:         uint64(m.ID),
			AgentType:  m.AgentType,
			Model:      m.ModelName,
			TokensUsed: m.TokensUsed,
			LatencyMs:  m.LatencyMs,
			Status:     m.Status,
			CreatedAt:  m.CreatedAt,
		}
	}
	return logs, total, nil
}

func (r *adminRepo) GetAICallLog(ctx context.Context, id uint64) (*biz.AICallLogDetail, error) {
	var m model.AICallLog
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &biz.AICallLogDetail{
		ID:              uint64(m.ID),
		TraceID:         m.TraceID,
		Source:          m.Source,
		Scene:           m.Scene,
		Provider:        m.Provider,
		Model:           m.ModelName,
		UserInput:       m.UserInput,
		ModelOutput:     m.ModelOutput,
		ModelError:      m.ModelError,
		LatencyMs:       m.LatencyMs,
		IsSuccess:       m.IsSuccess,
		InputTokens:     m.InputTokens,
		OutputTokens:    m.OutputTokens,
		RenderedPrompt:  m.RenderedPrompt,
		RequestMessages: m.RequestMessages,
		RuntimeConfig:   m.RuntimeConfig,
		CreatedAt:       m.CreatedAt,
	}, nil
}

// ==================== Live2D 管理 ====================

func (r *adminRepo) ListLive2DModels(ctx context.Context) ([]*biz.Live2DModelRecord, error) {
	var models_ []model.Live2DModel
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&models_).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.Live2DModelRecord, len(models_))
	for i, m := range models_ {
		industryID := uint64(0)
		if m.IndustryID != nil {
			industryID = uint64(*m.IndustryID)
		}
		ttsConfigID := uint64(0)
		if m.TTSConfigID != nil {
			ttsConfigID = uint64(*m.TTSConfigID)
		}
		result[i] = &biz.Live2DModelRecord{
			ID:           uint64(m.ID),
			Name:         m.Name,
			IndustryID:   industryID,
			Scene:        m.Scene,
			ModelURL:     m.ModelURL,
			ThumbnailURL: m.ThumbnailURL,
			ConfigJSON:   m.ConfigJSON,
			TTSConfigID:  ttsConfigID,
			IsActive:     m.IsActive,
			CreatedAt:    m.CreatedAt,
		}
	}
	return result, nil
}

// CreateLive2DModel 创建一条 Live2D 模型记录，并将数据库生成的主键回填到领域对象。
func (r *adminRepo) CreateLive2DModel(ctx context.Context, m *biz.Live2DModelRecord) error {
	model_ := &model.Live2DModel{
		Name:         m.Name,
		Scene:        m.Scene,
		ModelURL:     m.ModelURL,
		ThumbnailURL: m.ThumbnailURL,
		ConfigJSON:   m.ConfigJSON,
		IsActive:     m.IsActive,
	}
	if m.IndustryID > 0 {
		indID := uint(m.IndustryID)
		model_.IndustryID = &indID
	}
	if m.TTSConfigID > 0 {
		ttsID := uint(m.TTSConfigID)
		model_.TTSConfigID = &ttsID
	}
	if err := r.db.WithContext(ctx).
		Select("Name", "Scene", "ModelURL", "ThumbnailURL", "ConfigJSON", "TTSConfigID", "IndustryID", "IsActive").
		Create(model_).Error; err != nil {
		return err
	}
	if !m.IsActive {
		if err := r.db.WithContext(ctx).Model(model_).Update("is_active", false).Error; err != nil {
			return err
		}
		model_.IsActive = false
	}
	m.ID = uint64(model_.ID)
	return nil
}

func (r *adminRepo) UpdateLive2DModel(ctx context.Context, m *biz.Live2DModelRecord) error {
	updates := map[string]interface{}{
		"name":          m.Name,
		"scene":         m.Scene,
		"model_url":     m.ModelURL,
		"thumbnail_url": m.ThumbnailURL,
		"config_json":   m.ConfigJSON,
		"is_active":     m.IsActive,
	}
	if m.IndustryID > 0 {
		indID := uint(m.IndustryID)
		updates["industry_id"] = &indID
	} else {
		updates["industry_id"] = nil
	}
	if m.TTSConfigID > 0 {
		ttsID := uint(m.TTSConfigID)
		updates["tts_config_id"] = &ttsID
	} else {
		updates["tts_config_id"] = nil
	}
	return r.db.WithContext(ctx).Model(&model.Live2DModel{}).Where("id = ?", m.ID).Updates(updates).Error
}

func (r *adminRepo) DeleteLive2DModel(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Live2DModel{}, id).Error
}

// ==================== TTS 管理 ====================

func (r *adminRepo) ListTTSConfigs(ctx context.Context) ([]*biz.TTSConfigRecord, error) {
	var models_ []model.TTSConfig
	if err := r.db.WithContext(ctx).Order("sort_order ASC, created_at DESC").Find(&models_).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.TTSConfigRecord, len(models_))
	for i, m := range models_ {
		result[i] = &biz.TTSConfigRecord{
			ID:             uint64(m.ID),
			Name:           m.Name,
			Engine:         m.Engine,
			VoiceID:        m.VoiceID,
			AuthConfigJSON: m.AuthConfigJSON,
			ParamsJSON:     m.ParamsJSON,
			IsActive:       m.IsActive,
			SortOrder:      int32(m.SortOrder),
			CreatedAt:      m.CreatedAt,
		}
	}
	return result, nil
}

func (r *adminRepo) CreateTTSConfig(ctx context.Context, t *biz.TTSConfigRecord) error {
	m := &model.TTSConfig{
		Name:           t.Name,
		Engine:         t.Engine,
		VoiceID:        t.VoiceID,
		AuthConfigJSON: t.AuthConfigJSON,
		ParamsJSON:     t.ParamsJSON,
		IsActive:       t.IsActive,
		SortOrder:      int(t.SortOrder),
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateTTSConfig(ctx context.Context, t *biz.TTSConfigRecord) error {
	return r.db.WithContext(ctx).Model(&model.TTSConfig{}).Where("id = ?", t.ID).Updates(map[string]interface{}{
		"name":             t.Name,
		"engine":           t.Engine,
		"voice_id":         t.VoiceID,
		"auth_config_json": t.AuthConfigJSON,
		"params_json":      t.ParamsJSON,
		"is_active":        t.IsActive,
		"sort_order":       int(t.SortOrder),
	}).Error
}

func (r *adminRepo) DeleteTTSConfig(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.TTSConfig{}, id).Error
}

// ==================== Scraper 管理 ====================

// CreateScraperTask 创建 scraper_tasks 任务记录，并完整保存异步任务载荷快照。
func (r *adminRepo) CreateScraperTask(ctx context.Context, task *biz.ScraperTaskRecord) error {
	m := &model.ScraperTask{
		TaskType:      task.TaskType,
		SourceURL:     task.SourceURL,
		SourceTitle:   task.SourceTitle,
		Source:        task.Source,
		Status:        task.Status,
		PayloadJSON:   task.PayloadJSON,
		ResultJSON:    task.ResultJSON,
		QuestionCount: task.QuestionCount,
		ImportedCount: task.ImportedCount,
		RetryCount:    task.RetryCount,
		StartedAt:     task.StartedAt,
		FinishedAt:    task.FinishedAt,
		ErrorMsg:      task.ErrorMsg,
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	task.ID = uint64(m.ID)
	task.CreatedAt = m.CreatedAt
	task.UpdatedAt = m.UpdatedAt
	return nil
}

// ListScraperTasks 按筛选条件分页查询 scraper_tasks，并返回结果快照字段。
func (r *adminRepo) ListScraperTasks(ctx context.Context, page, pageSize int32, status, taskType string) ([]*biz.ScraperTaskRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.ScraperTask{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	var total int64
	query.Count(&total)

	var models_ []model.ScraperTask
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&models_).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*biz.ScraperTaskRecord, len(models_))
	for i, m := range models_ {
		result[i] = &biz.ScraperTaskRecord{
			ID:            uint64(m.ID),
			TaskType:      m.TaskType,
			SourceURL:     m.SourceURL,
			SourceTitle:   m.SourceTitle,
			Source:        m.Source,
			Status:        m.Status,
			PayloadJSON:   m.PayloadJSON,
			ResultJSON:    m.ResultJSON,
			QuestionCount: m.QuestionCount,
			ImportedCount: m.ImportedCount,
			RetryCount:    m.RetryCount,
			StartedAt:     m.StartedAt,
			FinishedAt:    m.FinishedAt,
			ErrorMsg:      m.ErrorMsg,
			CreatedAt:     m.CreatedAt,
			UpdatedAt:     m.UpdatedAt,
		}
	}
	return result, total, nil
}

// GetScraperTask 按主键读取单条 scraper_tasks 记录，供任务详情与 SSE 轮询使用。
func (r *adminRepo) GetScraperTask(ctx context.Context, id uint64) (*biz.ScraperTaskRecord, error) {
	var m model.ScraperTask
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &biz.ScraperTaskRecord{
		ID:            uint64(m.ID),
		TaskType:      m.TaskType,
		SourceURL:     m.SourceURL,
		SourceTitle:   m.SourceTitle,
		Source:        m.Source,
		Status:        m.Status,
		PayloadJSON:   m.PayloadJSON,
		ResultJSON:    m.ResultJSON,
		QuestionCount: m.QuestionCount,
		ImportedCount: m.ImportedCount,
		RetryCount:    m.RetryCount,
		StartedAt:     m.StartedAt,
		FinishedAt:    m.FinishedAt,
		ErrorMsg:      m.ErrorMsg,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}, nil
}

// UpdateScraperTask 更新 scraper_tasks 的执行状态、进度与结果快照。
func (r *adminRepo) UpdateScraperTask(ctx context.Context, task *biz.ScraperTaskRecord) error {
	return r.db.WithContext(ctx).Model(&model.ScraperTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":         task.Status,
		"payload_json":   task.PayloadJSON,
		"result_json":    task.ResultJSON,
		"question_count": task.QuestionCount,
		"imported_count": task.ImportedCount,
		"retry_count":    task.RetryCount,
		"started_at":     task.StartedAt,
		"finished_at":    task.FinishedAt,
		"error_msg":      task.ErrorMsg,
	}).Error
}

// ListScraperSources 返回可用的爬虫数据源列表。
// 当前使用硬编码源配置，与单体 HTTPProvider 保持一致。
func (r *adminRepo) ListScraperSources(_ context.Context) ([]*biz.ScraperSourceRecord, error) {
	return []*biz.ScraperSourceRecord{
		{Name: "niuke", Label: "牛客网", BaseURL: "https://www.nowcoder.com", IsActive: true},
		{Name: "leetcode", Label: "LeetCode", BaseURL: "https://leetcode.cn", IsActive: true},
		{Name: "juejin", Label: "掘金", BaseURL: "https://juejin.cn", IsActive: true},
	}, nil
}

// ==================== RAG 文档管理 ====================

func (r *adminRepo) ListRAGDocuments(ctx context.Context, page, pageSize int32, collection, docType, keyword, syncStatus string) ([]*biz.RAGDocumentRecord, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.RAGDocument{})
	if collection != "" {
		query = query.Where("collection = ?", collection)
	}
	if docType != "" {
		query = query.Where("doc_type = ?", docType)
	}
	if keyword != "" {
		query = query.Where("title ILIKE ? OR content ILIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if syncStatus != "" {
		query = query.Where("sync_status = ?", syncStatus)
	}

	var total int64
	query.Count(&total)

	var models_ []model.RAGDocument
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(int(offset)).Limit(int(pageSize)).Find(&models_).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*biz.RAGDocumentRecord, len(models_))
	for i, m := range models_ {
		result[i] = &biz.RAGDocumentRecord{
			ID:         uint64(m.ID),
			Collection: m.Collection,
			DocType:    m.DocType,
			Title:      m.Title,
			Content:    m.Content,
			Metadata:   m.Metadata,
			VectorID:   m.VectorID,
			SyncStatus: m.SyncStatus,
			IsActive:   m.IsActive,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
		}
	}
	return result, total, nil
}

func (r *adminRepo) GetRAGDocument(ctx context.Context, id uint64) (*biz.RAGDocumentRecord, error) {
	var m model.RAGDocument
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return &biz.RAGDocumentRecord{
		ID:         uint64(m.ID),
		Collection: m.Collection,
		DocType:    m.DocType,
		Title:      m.Title,
		Content:    m.Content,
		Metadata:   m.Metadata,
		VectorID:   m.VectorID,
		SyncStatus: m.SyncStatus,
		IsActive:   m.IsActive,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}, nil
}

func (r *adminRepo) CreateRAGDocument(ctx context.Context, doc *biz.RAGDocumentRecord) error {
	m := &model.RAGDocument{
		Collection: doc.Collection,
		DocType:    doc.DocType,
		Title:      doc.Title,
		Content:    doc.Content,
		Metadata:   doc.Metadata,
		SyncStatus: "pending",
		IsActive:   doc.IsActive,
	}
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *adminRepo) UpdateRAGDocument(ctx context.Context, doc *biz.RAGDocumentRecord) error {
	updates := map[string]interface{}{
		"collection":  doc.Collection,
		"doc_type":    doc.DocType,
		"title":       doc.Title,
		"content":     doc.Content,
		"metadata":    doc.Metadata,
		"is_active":   doc.IsActive,
		"sync_status": "pending", // 修改后重新标记为待同步
	}
	return r.db.WithContext(ctx).Model(&model.RAGDocument{}).Where("id = ?", doc.ID).Updates(updates).Error
}

func (r *adminRepo) DeleteRAGDocument(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.RAGDocument{}, id).Error
}

func (r *adminRepo) BatchCreateRAGDocuments(ctx context.Context, docs []*biz.RAGDocumentRecord) (int, int, []string) {
	success, fail := 0, 0
	var errors []string
	for _, doc := range docs {
		if err := r.CreateRAGDocument(ctx, doc); err != nil {
			fail++
			errors = append(errors, err.Error())
		} else {
			success++
		}
	}
	return success, fail, errors
}

func (r *adminRepo) GetRAGDocumentStats(ctx context.Context, collection string) (map[string]int64, error) {
	query := r.db.WithContext(ctx).Model(&model.RAGDocument{})
	if collection != "" {
		query = query.Where("collection = ?", collection)
	}

	stats := make(map[string]int64)
	var results []struct {
		DocType string
		Count   int64
	}
	if err := query.Select("doc_type, COUNT(*) as count").Group("doc_type").Find(&results).Error; err != nil {
		return nil, err
	}
	for _, r := range results {
		stats[r.DocType] = r.Count
	}
	return stats, nil
}

func (r *adminRepo) GetPendingSyncRAGDocuments(ctx context.Context, limit int) ([]*biz.RAGDocumentRecord, error) {
	var models_ []model.RAGDocument
	if err := r.db.WithContext(ctx).Where("sync_status = ?", "pending").Limit(limit).Find(&models_).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.RAGDocumentRecord, len(models_))
	for i, m := range models_ {
		result[i] = &biz.RAGDocumentRecord{
			ID:         uint64(m.ID),
			Collection: m.Collection,
			DocType:    m.DocType,
			Title:      m.Title,
			Content:    m.Content,
			Metadata:   m.Metadata,
			VectorID:   m.VectorID,
			SyncStatus: m.SyncStatus,
			IsActive:   m.IsActive,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
		}
	}
	return result, nil
}

func (r *adminRepo) UpdateRAGDocumentSyncStatus(ctx context.Context, id uint64, status, vectorID string) error {
	return r.db.WithContext(ctx).Model(&model.RAGDocument{}).Where("id = ?", id).Updates(map[string]interface{}{
		"sync_status": status,
		"vector_id":   vectorID,
	}).Error
}

// ==================== 题目查询（供 RAG 索引使用） ====================

func (r *adminRepo) ListAllQuestions(ctx context.Context, industryID uint64, pageSize int, offset int) ([]*biz.QuestionRecord, error) {
	query := r.db.WithContext(ctx).Model(&model.Question{})
	if industryID > 0 {
		query = query.Where("industry_id = ?", industryID)
	}
	var questions []model.Question
	if err := query.Offset(offset).Limit(pageSize).Find(&questions).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.QuestionRecord, len(questions))
	for i, q := range questions {
		result[i] = &biz.QuestionRecord{
			ID:              uint64(q.ID),
			CategoryID:      uint64(q.CategoryID),
			IndustryID:      uint64(q.IndustryID),
			Type:            q.Type,
			Difficulty:      q.Difficulty,
			Title:           q.Title,
			Content:         q.Content,
			OptionsJSON:     q.OptionsJSON,
			Answer:          q.Answer,
			Explanation:     q.Explanation,
			SolutionJSON:    q.SolutionJSON,
			JudgeConfigJSON: q.JudgeConfigJSON,
			Tags:            q.Tags,
			IsActive:        q.IsActive,
			CreatedAt:       q.CreatedAt,
		}
	}
	return result, nil
}

func (r *adminRepo) GetQuestionsByIDs(ctx context.Context, ids []uint64) ([]*biz.QuestionRecord, error) {
	var questions []model.Question
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&questions).Error; err != nil {
		return nil, err
	}
	result := make([]*biz.QuestionRecord, len(questions))
	for i, q := range questions {
		result[i] = &biz.QuestionRecord{
			ID:              uint64(q.ID),
			CategoryID:      uint64(q.CategoryID),
			IndustryID:      uint64(q.IndustryID),
			Type:            q.Type,
			Difficulty:      q.Difficulty,
			Title:           q.Title,
			Content:         q.Content,
			OptionsJSON:     q.OptionsJSON,
			Answer:          q.Answer,
			Explanation:     q.Explanation,
			SolutionJSON:    q.SolutionJSON,
			JudgeConfigJSON: q.JudgeConfigJSON,
			Tags:            q.Tags,
			IsActive:        q.IsActive,
			CreatedAt:       q.CreatedAt,
		}
	}
	return result, nil
}
