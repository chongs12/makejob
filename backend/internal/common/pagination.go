// Package common 提供分页相关工具函数
package common

import "math"

// Pagination 分页计算工具
type Pagination struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
	HasNext    bool  `json:"has_next"`
	HasPrev    bool  `json:"has_prev"`
}

// NewPagination 创建分页对象
func NewPagination(page, pageSize int, total int64) *Pagination {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))

	return &Pagination{
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// Offset 计算数据库查询偏移量
func (p *Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

// Limit 获取查询限制数量
func (p *Pagination) Limit() int {
	return p.PageSize
}

// Paginate 对切片进行分页
func Paginate(data []interface{}, page, pageSize int) []interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	start := (page - 1) * pageSize
	if start >= len(data) {
		return []interface{}{}
	}

	end := start + pageSize
	if end > len(data) {
		end = len(data)
	}

	return data[start:end]
}

// CalculateTotalPages 计算总页数
func CalculateTotalPages(total int64, pageSize int) int {
	if pageSize <= 0 {
		pageSize = 10
	}
	return int(math.Ceil(float64(total) / float64(pageSize)))
}
