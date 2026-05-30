package repository

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"makejob-backend/internal/model"
)

// RAGDocumentListParams RAG文档列表查询参数
type RAGDocumentListParams struct {
	Page       int
	PageSize   int
	Collection string
	DocType    string
	Keyword    string
	SyncStatus string
	IsActive   *bool
}

// RAGDocumentRepository RAG文档数据访问接口
type RAGDocumentRepository interface {
	List(ctx context.Context, params RAGDocumentListParams) ([]model.RAGDocument, int64, error)
	GetByID(ctx context.Context, id uint) (*model.RAGDocument, error)
	GetByIDs(ctx context.Context, ids []uint) ([]model.RAGDocument, error)
	GetPendingSync(ctx context.Context, collection string, limit int) ([]model.RAGDocument, error)
	Create(ctx context.Context, doc *model.RAGDocument) error
	Update(ctx context.Context, doc *model.RAGDocument) error
	Delete(ctx context.Context, id uint) error
	BatchCreate(ctx context.Context, docs []model.RAGDocument) error
	UpdateSyncStatus(ctx context.Context, id uint, status string, vectorID string) error
	CountByType(ctx context.Context, collection string) (map[string]int64, error)
}

// ragDocumentRepository RAG文档数据访问实现
type ragDocumentRepository struct {
	db *gorm.DB
}

// NewRAGDocumentRepository 创建RAG文档仓库实例
func NewRAGDocumentRepository(db *gorm.DB) RAGDocumentRepository {
	return &ragDocumentRepository{db: db}
}

// List 获取RAG文档列表
func (r *ragDocumentRepository) List(ctx context.Context, params RAGDocumentListParams) ([]model.RAGDocument, int64, error) {
	var docs []model.RAGDocument
	var total int64

	query := r.db.WithContext(ctx).Model(&model.RAGDocument{})

	// 应用筛选条件
	if params.Collection != "" {
		query = query.Where("collection = ?", params.Collection)
	}
	if params.DocType != "" {
		query = query.Where("doc_type = ?", params.DocType)
	}
	if params.SyncStatus != "" {
		query = query.Where("sync_status = ?", params.SyncStatus)
	}
	if params.IsActive != nil {
		query = query.Where("is_active = ?", *params.IsActive)
	}
	if params.Keyword != "" {
		keyword := "%" + params.Keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", keyword, keyword)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("统计RAG文档总数失败: %w", err)
	}

	// 规范化分页参数
	page := params.Page
	if page <= 0 {
		page = 1
	}
	pageSize := params.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	// 查询数据
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Limit(pageSize).Offset(offset).Find(&docs).Error; err != nil {
		return nil, 0, fmt.Errorf("查询RAG文档列表失败: %w", err)
	}

	return docs, total, nil
}

// GetByID 根据ID获取RAG文档
func (r *ragDocumentRepository) GetByID(ctx context.Context, id uint) (*model.RAGDocument, error) {
	var doc model.RAGDocument
	if err := r.db.WithContext(ctx).First(&doc, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("查询RAG文档失败: %w", err)
	}
	return &doc, nil
}

// GetByIDs 根据ID列表获取RAG文档
func (r *ragDocumentRepository) GetByIDs(ctx context.Context, ids []uint) ([]model.RAGDocument, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	var docs []model.RAGDocument
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&docs).Error; err != nil {
		return nil, fmt.Errorf("查询RAG文档列表失败: %w", err)
	}
	return docs, nil
}

// GetPendingSync 获取待同步的RAG文档
func (r *ragDocumentRepository) GetPendingSync(ctx context.Context, collection string, limit int) ([]model.RAGDocument, error) {
	if limit <= 0 {
		limit = 100
	}

	var docs []model.RAGDocument
	query := r.db.WithContext(ctx).Where("sync_status = ? AND is_active = ?", model.RAGSyncStatusPending, true)
	if collection != "" {
		query = query.Where("collection = ?", collection)
	}

	if err := query.Order("id ASC").Limit(limit).Find(&docs).Error; err != nil {
		return nil, fmt.Errorf("查询待同步RAG文档失败: %w", err)
	}
	return docs, nil
}

// Create 创建RAG文档
func (r *ragDocumentRepository) Create(ctx context.Context, doc *model.RAGDocument) error {
	if err := r.db.WithContext(ctx).Create(doc).Error; err != nil {
		return fmt.Errorf("创建RAG文档失败: %w", err)
	}
	return nil
}

// Update 更新RAG文档
func (r *ragDocumentRepository) Update(ctx context.Context, doc *model.RAGDocument) error {
	if err := r.db.WithContext(ctx).Save(doc).Error; err != nil {
		return fmt.Errorf("更新RAG文档失败: %w", err)
	}
	return nil
}

// Delete 删除RAG文档（软删除）
func (r *ragDocumentRepository) Delete(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Delete(&model.RAGDocument{}, id).Error; err != nil {
		return fmt.Errorf("删除RAG文档失败: %w", err)
	}
	return nil
}

// BatchCreate 批量创建RAG文档
func (r *ragDocumentRepository) BatchCreate(ctx context.Context, docs []model.RAGDocument) error {
	if len(docs) == 0 {
		return nil
	}

	if err := r.db.WithContext(ctx).CreateInBatches(docs, 100).Error; err != nil {
		return fmt.Errorf("批量创建RAG文档失败: %w", err)
	}
	return nil
}

// UpdateSyncStatus 更新RAG文档同步状态
func (r *ragDocumentRepository) UpdateSyncStatus(ctx context.Context, id uint, status string, vectorID string) error {
	updates := map[string]interface{}{
		"sync_status": status,
	}
	if vectorID != "" {
		updates["vector_id"] = vectorID
	}

	if err := r.db.WithContext(ctx).Model(&model.RAGDocument{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新RAG文档同步状态失败: %w", err)
	}
	return nil
}

// CountByType 按文档类型统计数量
func (r *ragDocumentRepository) CountByType(ctx context.Context, collection string) (map[string]int64, error) {
	type Result struct {
		DocType string
		Count   int64
	}

	var results []Result
	query := r.db.WithContext(ctx).Model(&model.RAGDocument{}).
		Select("doc_type, COUNT(*) as count").
		Where("is_active = ?", true)

	if collection != "" {
		query = query.Where("collection = ?", collection)
	}

	if err := query.Group("doc_type").Find(&results).Error; err != nil {
		return nil, fmt.Errorf("统计RAG文档类型失败: %w", err)
	}

	stats := make(map[string]int64)
	for _, r := range results {
		stats[r.DocType] = r.Count
	}

	// 确保所有类型都有值
	for _, docType := range []string{model.RAGDocTypeTechDoc, model.RAGDocTypeInterviewExp, model.RAGDocTypeJobRequire} {
		if _, ok := stats[docType]; !ok {
			stats[docType] = 0
		}
	}

	return stats, nil
}

// normalizeRAGDocumentListParams 规范化查询参数
func normalizeRAGDocumentListParams(params RAGDocumentListParams) RAGDocumentListParams {
	params.Collection = strings.TrimSpace(params.Collection)
	params.DocType = strings.TrimSpace(params.DocType)
	params.Keyword = strings.TrimSpace(params.Keyword)
	params.SyncStatus = strings.TrimSpace(params.SyncStatus)
	return params
}
