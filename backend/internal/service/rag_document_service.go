package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"makejob-backend/internal/common"
	"makejob-backend/internal/model"
	"makejob-backend/internal/rag"
	"makejob-backend/internal/repository"
)

// RAGDocumentService RAG文档服务接口
type RAGDocumentService interface {
	ListDocuments(ctx context.Context, params repository.RAGDocumentListParams) (*common.PageResult, error)
	GetDocument(ctx context.Context, id uint) (*model.RAGDocument, error)
	CreateDocument(ctx context.Context, req *CreateRAGDocumentRequest) (*model.RAGDocument, error)
	UpdateDocument(ctx context.Context, id uint, req *UpdateRAGDocumentRequest) error
	DeleteDocument(ctx context.Context, id uint) error
	BatchImportDocuments(ctx context.Context, req *BatchImportRAGDocumentRequest) (*BatchImportRAGDocumentResponse, error)
	SyncToVectorDB(ctx context.Context, ids []uint) error
	SyncAllPending(ctx context.Context) error
	GetDocumentStats(ctx context.Context, collection string) (map[string]int64, error)
}

// CreateRAGDocumentRequest 创建RAG文档请求
type CreateRAGDocumentRequest struct {
	Collection string            `json:"collection" binding:"required"`
	DocType    string            `json:"doc_type" binding:"required"`
	Title      string            `json:"title" binding:"required"`
	Content    string            `json:"content" binding:"required"`
	Metadata   map[string]any    `json:"metadata"`
}

// UpdateRAGDocumentRequest 更新RAG文档请求
type UpdateRAGDocumentRequest struct {
	Collection *string           `json:"collection"`
	DocType    *string           `json:"doc_type"`
	Title      *string           `json:"title"`
	Content    *string           `json:"content"`
	Metadata   map[string]any    `json:"metadata"`
	IsActive   *bool             `json:"is_active"`
}

// BatchImportRAGDocumentRequest 批量导入RAG文档请求
type BatchImportRAGDocumentRequest struct {
	Collection string                   `json:"collection" binding:"required"`
	DocType    string                   `json:"doc_type" binding:"required"`
	Documents  []BatchImportDocItem     `json:"documents" binding:"required,min=1"`
}

// BatchImportDocItem 批量导入文档项
type BatchImportDocItem struct {
	Title    string         `json:"title" binding:"required"`
	Content  string         `json:"content" binding:"required"`
	Metadata map[string]any `json:"metadata"`
}

// BatchImportRAGDocumentResponse 批量导入RAG文档响应
type BatchImportRAGDocumentResponse struct {
	Imported int    `json:"imported"`
	Failed   int    `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// ragDocumentService RAG文档服务实现
type ragDocumentService struct {
	repo       repository.RAGDocumentRepository
	ragService *rag.Service
}

// NewRAGDocumentService 创建RAG文档服务实例
func NewRAGDocumentService(repo repository.RAGDocumentRepository, ragService *rag.Service) RAGDocumentService {
	return &ragDocumentService{
		repo:       repo,
		ragService: ragService,
	}
}

// ListDocuments 获取RAG文档列表
func (s *ragDocumentService) ListDocuments(ctx context.Context, params repository.RAGDocumentListParams) (*common.PageResult, error) {
	docs, total, err := s.repo.List(ctx, params)
	if err != nil {
		return nil, err
	}

	return &common.PageResult{
		List:     docs,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}, nil
}

// GetDocument 获取RAG文档详情
func (s *ragDocumentService) GetDocument(ctx context.Context, id uint) (*model.RAGDocument, error) {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, common.NewBusinessError(common.CodeNotFound, "文档不存在")
	}
	return doc, nil
}

// CreateDocument 创建RAG文档
func (s *ragDocumentService) CreateDocument(ctx context.Context, req *CreateRAGDocumentRequest) (*model.RAGDocument, error) {
	// 验证文档类型
	if !isValidRAGDocType(req.DocType) {
		return nil, common.NewBusinessError(common.CodeBadRequest, "无效的文档类型")
	}

	// 序列化元数据
	metadataJSON := "{}"
	if req.Metadata != nil {
		data, err := json.Marshal(req.Metadata)
		if err != nil {
			return nil, common.NewBusinessError(common.CodeBadRequest, "元数据格式错误")
		}
		metadataJSON = string(data)
	}

	doc := &model.RAGDocument{
		Collection: req.Collection,
		DocType:    normalizeRAGDocType(req.DocType),
		Title:      req.Title,
		Content:    req.Content,
		Metadata:   metadataJSON,
		SyncStatus: model.RAGSyncStatusPending,
		IsActive:   true,
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocument 更新RAG文档
func (s *ragDocumentService) UpdateDocument(ctx context.Context, id uint, req *UpdateRAGDocumentRequest) error {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return common.NewBusinessError(common.CodeNotFound, "文档不存在")
	}

	// 更新字段
	if req.Collection != nil {
		doc.Collection = *req.Collection
	}
	if req.DocType != nil {
		if !isValidRAGDocType(*req.DocType) {
			return common.NewBusinessError(common.CodeBadRequest, "无效的文档类型")
		}
		doc.DocType = *req.DocType
	}
	if req.Title != nil {
		doc.Title = *req.Title
	}
	if req.Content != nil {
		doc.Content = *req.Content
	}
	if req.Metadata != nil {
		data, err := json.Marshal(req.Metadata)
		if err != nil {
			return common.NewBusinessError(common.CodeBadRequest, "元数据格式错误")
		}
		doc.Metadata = string(data)
	}
	if req.IsActive != nil {
		doc.IsActive = *req.IsActive
	}

	// 标记为待同步
	doc.SyncStatus = model.RAGSyncStatusPending

	return s.repo.Update(ctx, doc)
}

// DeleteDocument 删除RAG文档
func (s *ragDocumentService) DeleteDocument(ctx context.Context, id uint) error {
	doc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if doc == nil {
		return common.NewBusinessError(common.CodeNotFound, "文档不存在")
	}

	// 如果已同步，从向量库删除
	if doc.SyncStatus == model.RAGSyncStatusSynced && doc.VectorID != "" && s.ragService != nil {
		if err := s.ragService.DeleteByIDs(ctx, []string{doc.VectorID}); err != nil {
			// 记录错误但不阻止删除
			fmt.Printf("从向量库删除文档失败: %v\n", err)
		}
	}

	return s.repo.Delete(ctx, id)
}

// BatchImportDocuments 批量导入RAG文档
func (s *ragDocumentService) BatchImportDocuments(ctx context.Context, req *BatchImportRAGDocumentRequest) (*BatchImportRAGDocumentResponse, error) {
	// 验证文档类型
	if !isValidRAGDocType(req.DocType) {
		return nil, common.NewBusinessError(common.CodeBadRequest, "无效的文档类型")
	}

	response := &BatchImportRAGDocumentResponse{}
	var docs []model.RAGDocument

	for _, item := range req.Documents {
		metadataJSON := "{}"
		if item.Metadata != nil {
			data, err := json.Marshal(item.Metadata)
			if err != nil {
				response.Failed++
				response.Errors = append(response.Errors, fmt.Sprintf("文档 %q 元数据格式错误", item.Title))
				continue
			}
			metadataJSON = string(data)
		}

		docs = append(docs, model.RAGDocument{
			Collection: req.Collection,
			DocType:    normalizeRAGDocType(req.DocType),
			Title:      item.Title,
			Content:    item.Content,
			Metadata:   metadataJSON,
			SyncStatus: model.RAGSyncStatusPending,
			IsActive:   true,
		})
	}

	if len(docs) > 0 {
		if err := s.repo.BatchCreate(ctx, docs); err != nil {
			return nil, err
		}
		response.Imported = len(docs)
	}

	return response, nil
}

// SyncToVectorDB 同步指定文档到向量库
func (s *ragDocumentService) SyncToVectorDB(ctx context.Context, ids []uint) error {
	if s.ragService == nil {
		return common.NewBusinessError(common.CodeInternalError, "RAG服务未初始化")
	}

	docs, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}

	if len(docs) == 0 {
		return common.NewBusinessError(common.CodeNotFound, "未找到指定文档")
	}

	// 转换为RAG IndexDocument
	indexDocs := make([]rag.IndexDocument, 0, len(docs))
	for _, doc := range docs {
		if !doc.IsActive {
			continue
		}

		metadata := map[string]any{
			"doc_id":    doc.ID,
			"doc_type":  doc.DocType,
			"collection": doc.Collection,
		}

		// 合并自定义元数据
		if doc.Metadata != "" && doc.Metadata != "{}" {
			var customMeta map[string]any
			if err := json.Unmarshal([]byte(doc.Metadata), &customMeta); err == nil {
				for k, v := range customMeta {
					metadata[k] = v
				}
			}
		}

		indexDocs = append(indexDocs, rag.IndexDocument{
			ID:       fmt.Sprintf("doc-%d", doc.ID),
			Content:  doc.Title + "\n" + doc.Content,
			MetaData: metadata,
		})
	}

	if len(indexDocs) == 0 {
		return nil
	}

	// 调用RAG Service索引
	vectorIDs, err := s.ragService.Indexer().Index(ctx, indexDocs)
	if err != nil {
		// 更新所有文档状态为失败
		for _, doc := range docs {
			s.repo.UpdateSyncStatus(ctx, doc.ID, model.RAGSyncStatusFailed, "")
		}
		return fmt.Errorf("索引文档失败: %w", err)
	}

	// 更新同步状态
	for i, doc := range docs {
		vectorID := ""
		if i < len(vectorIDs) {
			vectorID = vectorIDs[i]
		}
		s.repo.UpdateSyncStatus(ctx, doc.ID, model.RAGSyncStatusSynced, vectorID)
	}

	return nil
}

// SyncAllPending 同步所有待同步文档
func (s *ragDocumentService) SyncAllPending(ctx context.Context) error {
	if s.ragService == nil {
		return common.NewBusinessError(common.CodeInternalError, "RAG服务未初始化")
	}

	// 分批获取待同步文档
	batchSize := 100
	for {
		docs, err := s.repo.GetPendingSync(ctx, "", batchSize)
		if err != nil {
			return err
		}

		if len(docs) == 0 {
			break
		}

		// 提取ID列表
		ids := make([]uint, 0, len(docs))
		for _, doc := range docs {
			ids = append(ids, doc.ID)
		}

		// 同步到向量库
		if err := s.SyncToVectorDB(ctx, ids); err != nil {
			return err
		}

		// 如果不足一批，说明已经全部处理完
		if len(docs) < batchSize {
			break
		}
	}

	return nil
}

// GetDocumentStats 获取文档统计信息
func (s *ragDocumentService) GetDocumentStats(ctx context.Context, collection string) (map[string]int64, error) {
	return s.repo.CountByType(ctx, collection)
}

// normalizeRAGDocType 规范文档类型为小写
func normalizeRAGDocType(docType string) string {
	return strings.ToLower(strings.TrimSpace(docType))
}

// isValidRAGDocType 验证文档类型是否有效
func isValidRAGDocType(docType string) bool {
	validTypes := []string{
		model.RAGDocTypeTechDoc,
		model.RAGDocTypeInterviewExp,
		model.RAGDocTypeJobRequire,
	}
	normalized := normalizeRAGDocType(docType)
	for _, t := range validTypes {
		if t == normalized {
			return true
		}
	}
	return false
}
