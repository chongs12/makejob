package model

// StudyLog 学习记录 GORM model
type StudyLog struct {
	BaseModel
	UserID           uint64 `gorm:"index;not null"`
	DateKey          string `gorm:"size:10;index;not null"`
	PlanID           uint64
	Summary          string `gorm:"size:500"`
	Action           string `gorm:"size:50;index;not null"`
	RefID            uint64
	RefType          string `gorm:"size:50"`
	DurationMinutes  int32
	Source           string `gorm:"size:20"`
	FocusTaskTitle   string `gorm:"size:200"`
	CompletedCount   int32
	SkippedCount     int32
	CompletedTitles  string `gorm:"type:text"`
	SkippedTitles    string `gorm:"type:text"`
	LatestActionText string `gorm:"size:500"`
}

func (StudyLog) TableName() string { return "study_logs" }
