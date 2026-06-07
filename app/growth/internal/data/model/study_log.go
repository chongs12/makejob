package model

// StudyLog 学习记录 GORM model（FIX G3: 使用 BaseModel 替代 gorm.Model，移除冗余 CreatedAt）
type StudyLog struct {
	BaseModel
	UserID          uint64 `gorm:"index;not null"`
	DateKey         string `gorm:"size:10;index;not null"`
	PlanID          uint64
	Summary         string `gorm:"size:500"`
	Action          string `gorm:"size:50;index;not null"`
	RefID           uint64
	RefType         string `gorm:"size:50"`
	DurationMinutes int32
	Source          string `gorm:"size:20"`
}

func (StudyLog) TableName() string { return "study_logs" }
