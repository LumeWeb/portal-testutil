package testutil

import (
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

// PaginationHelper provides utilities for testing pagination
type PaginationHelper struct{}

// NewPaginationHelper creates a new pagination helper
func NewPaginationHelper() *PaginationHelper {
	return &PaginationHelper{}
}

// DefaultPagination returns a default pagination configuration
func (p *PaginationHelper) DefaultPagination() queryutil.Pagination {
	return queryutil.Pagination{
		Start:    0,
		End:      100,
		PageSize: 100,
	}
}

// CreatePagination creates a pagination configuration
func (p *PaginationHelper) CreatePagination(page, pageSize int) queryutil.Pagination {
	start := (page - 1) * pageSize
	end := start + pageSize
	return queryutil.Pagination{
		Start:    start,
		End:      end,
		PageSize: pageSize,
	}
}

// FirstPage returns pagination for the first page
func (p *PaginationHelper) FirstPage(pageSize int) queryutil.Pagination {
	return p.CreatePagination(1, pageSize)
}

// NextPage returns pagination for the next page
func (p *PaginationHelper) NextPage(currentPagination queryutil.Pagination) queryutil.Pagination {
	nextPage := (currentPagination.Start / currentPagination.PageSize) + 2
	return p.CreatePagination(nextPage, currentPagination.PageSize)
}

// PreviousPage returns pagination for the previous page
func (p *PaginationHelper) PreviousPage(currentPagination queryutil.Pagination) queryutil.Pagination {
	prevPage := (currentPagination.Start / currentPagination.PageSize)
	if prevPage < 1 {
		prevPage = 1
	}
	return p.CreatePagination(prevPage, currentPagination.PageSize)
}

// CalculateTotalPages calculates the total number of pages
func (p *PaginationHelper) CalculateTotalPages(totalItems int64, pageSize int) int {
	if totalItems == 0 {
		return 0
	}

	pages := int(totalItems) / pageSize
	if int(totalItems)%pageSize > 0 {
		pages++
	}
	return pages
}

// IsLastPage checks if the current pagination is on the last page
func (p *PaginationHelper) IsLastPage(currentPagination queryutil.Pagination, totalItems int64) bool {
	totalPages := p.CalculateTotalPages(totalItems, currentPagination.PageSize)
	currentPage := (currentPagination.Start / currentPagination.PageSize) + 1
	return int(currentPage) >= totalPages
}

// GetPageNumber returns the current page number
func (p *PaginationHelper) GetPageNumber(pagination queryutil.Pagination) int {
	return (pagination.Start / pagination.PageSize) + 1
}

// LastPage returns pagination for the last page
func (p *PaginationHelper) LastPage(totalItems int64, pageSize int) queryutil.Pagination {
	totalPages := p.CalculateTotalPages(totalItems, pageSize)
	return p.CreatePagination(totalPages, pageSize)
}

// GetPageItems returns the number of items on the current page
func (p *PaginationHelper) GetPageItems(pagination queryutil.Pagination, totalItems int64) int64 {
	if p.IsLastPage(pagination, totalItems) {
		// For the last page, calculate remaining items
		_ = p.CalculateTotalPages(totalItems, pagination.PageSize)
		itemsOnLastPage := totalItems % int64(pagination.PageSize)
		if itemsOnLastPage == 0 && totalItems > 0 {
			// If the division is exact, the last page is full
			return int64(pagination.PageSize)
		}
		return itemsOnLastPage
	}
	return int64(pagination.PageSize)
}

// ApplyPaginationToQuery applies pagination parameters to a GORM query
func (p *PaginationHelper) ApplyPaginationToQuery(query *gorm.DB, pagination queryutil.Pagination) *gorm.DB {
	return query.Offset(pagination.Start).Limit(pagination.PageSize)
}

// CreatePaginationFromRequest creates pagination from request parameters
func (p *PaginationHelper) CreatePaginationFromRequest(page, pageSize int, maxPageSize int) queryutil.Pagination {
	// Ensure page is at least 1
	if page < 1 {
		page = 1
	}

	// Ensure page size is reasonable
	if pageSize <= 0 {
		pageSize = 10 // Default page size
	}

	if maxPageSize > 0 && pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return p.CreatePagination(page, pageSize)
}
