package pagination

// Params 分页参数
type Params struct {
	Page     int32
	PageSize int32
}

// Normalize 规范化分页参数
func (p *Params) Normalize() {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 20
	}
	if p.PageSize > 100 {
		p.PageSize = 100
	}
}

// Offset 计算偏移量
func (p *Params) Offset() int {
	return int((p.Page - 1) * p.PageSize)
}

// Limit 获取每页数量
func (p *Params) Limit() int {
	return int(p.PageSize)
}

// Result 分页结果
type Result struct {
	Total    int64
	Page     int32
	PageSize int32
}
