package model

// RAG文档类型常量
const (
	RAGDocTypeTechDoc      = "tech_doc"        // 技术文档
	RAGDocTypeInterviewExp = "interview_exp"   // 面经
	RAGDocTypeJobRequire   = "job_requirement" // 岗位要求
)

// RAG同步状态常量
const (
	RAGSyncStatusPending = "pending" // 待同步
	RAGSyncStatusSynced  = "synced"  // 已同步
	RAGSyncStatusFailed  = "failed"  // 同步失败
)

// RAGDocument RAG知识库文档
type RAGDocument struct {
	BaseModel
	Collection string `json:"collection" gorm:"size:100;not null;index;comment:所属Collection"`
	DocType    string `json:"doc_type" gorm:"size:50;not null;index;comment:文档类型"`
	Title      string `json:"title" gorm:"size:500;not null;comment:文档标题"`
	Content    string `json:"content" gorm:"type:text;not null;comment:文档内容"`
	Metadata   string `json:"metadata" gorm:"type:jsonb;comment:元数据JSON"`
	VectorID   string `json:"vector_id" gorm:"size:100;comment:Milvus中的文档ID"`
	SyncStatus string `json:"sync_status" gorm:"size:20;not null;default:pending;comment:同步状态"`
	IsActive   bool   `json:"is_active" gorm:"not null;default:true;comment:是否启用"`
}

func (RAGDocument) TableName() string { return "rag_documents" }
