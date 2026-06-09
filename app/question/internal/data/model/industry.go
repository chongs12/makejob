package model

// Industry 行业静态字典表模型。
// 该表以 code 作为稳定主键，采用静态配置治理，不引入软删除语义。
type Industry struct {
	Code string `gorm:"primaryKey;size:50"`
	Name string `gorm:"size:200;not null"`
	Icon string `gorm:"size:500"`
}

// TableName 返回行业字典表名。
func (Industry) TableName() string { return "industries" }
