package model

type Industry struct {
	Code string `gorm:"primaryKey;size:50"`
	Name string `gorm:"size:200;not null"`
	Icon string `gorm:"size:500"`
}

func (Industry) TableName() string { return "industries" }
