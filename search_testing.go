package testutil

import (
	"fmt"
	"strings"

	"github.com/DATA-DOG/go-sqlmock"
	"go.lumeweb.com/queryutil"
)

// SearchTestHelper provides utilities for testing search functionality
type SearchTestHelper struct {
	tc *DBTestContext
}

// NewSearchTestHelper creates a new search test helper
func NewSearchTestHelper(tc *DBTestContext) *SearchTestHelper {
	return &SearchTestHelper{tc: tc}
}

// ExpectSearchWithQueryUtil sets up expectations for a search operation using queryutil structures
func (h *SearchTestHelper) ExpectSearchWithQueryUtil(table string, filters []queryutil.Filter, sorts []queryutil.Sort,
	pagination queryutil.Pagination, count int64, rows *sqlmock.Rows) {

	// Build WHERE clause based on filters
	whereClause := h.buildWhereClauseFromFilters(filters)

	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Build query with sorting and pagination
	query := fmt.Sprintf("SELECT (.+) FROM `%s` WHERE %s", table, whereClause)

	// Add ORDER BY if sorts are specified
	if len(sorts) > 0 {
		var orderClauses []string
		for _, sort := range sorts {
			direction := "ASC"
			if sort.Order == queryutil.OrderDesc {
				direction = "DESC"
			}
			orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sort.Field, direction))
		}
		query += " ORDER BY " + strings.Join(orderClauses, ", ")
	}

	// Add LIMIT and OFFSET if pagination is specified
	if pagination.PageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", pagination.PageSize)
		if pagination.Start > 0 {
			query += fmt.Sprintf(" OFFSET %d", pagination.Start)
		}
	}

	// Setup rows expectation
	h.tc.mock.ExpectQuery(query).WillReturnRows(rows)
}

// buildWhereClauseFromFilters builds a WHERE clause from queryutil filters
func (h *SearchTestHelper) buildWhereClauseFromFilters(filters []queryutil.Filter) string {
	if len(filters) == 0 {
		return "1=1" // Default WHERE clause that matches everything
	}

	var conditions []string
	for _, filter := range filters {
		var condition string
		switch filter.Operator {
		case queryutil.OperatorEquals:
			condition = fmt.Sprintf("%s = '%v'", filter.Field, filter.Value)
		case queryutil.OperatorNotEquals:
			condition = fmt.Sprintf("%s <> '%v'", filter.Field, filter.Value)
		case queryutil.OperatorGTE:
			condition = fmt.Sprintf("%s >= %v", filter.Field, filter.Value)
		case queryutil.OperatorLTE:
			condition = fmt.Sprintf("%s <= %v", filter.Field, filter.Value)
		case queryutil.OperatorContains:
			condition = fmt.Sprintf("%s LIKE '%%%v%%'", filter.Field, filter.Value)
		default:
			// For any other operators, use a simple equality check
			condition = fmt.Sprintf("%s = '%v'", filter.Field, filter.Value)
		}
		conditions = append(conditions, condition)
	}

	return strings.Join(conditions, " AND ")
}

// ExpectSearch sets up expectations for a search operation
func (h *SearchTestHelper) ExpectSearch(table, query string, count int64, rows *sqlmock.Rows) {
	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Setup rows expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE", table)).
		WillReturnRows(rows)
}

// ExpectSearchWithFilters sets up expectations for a search operation with filters
func (h *SearchTestHelper) ExpectSearchWithFilters(table, query string, filters []queryutil.Filter, count int64, rows *sqlmock.Rows) {
	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Setup rows expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE", table)).
		WillReturnRows(rows)
}

// ExpectSearchError sets up expectations for a search operation that returns an error
func (h *SearchTestHelper) ExpectSearchError(table, query string, err error) {
	// Setup count expectation with error
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
		WillReturnError(err)
}

// ExpectTextSearch sets up expectations for a text search operation
// This is useful for searching text fields like name, description, etc.
func (h *SearchTestHelper) ExpectTextSearch(table string, searchFields []string, searchTerm string, count int64, rows *sqlmock.Rows) {
	// Build the WHERE clause for text search
	whereClause := h.buildTextSearchWhereClause(searchFields, searchTerm)

	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Setup rows expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(rows)
}

// ExpectGlobalSearch sets up expectations for a global search operation across multiple tables
func (h *SearchTestHelper) ExpectGlobalSearch(tables []string, searchTerm string, counts []int64, rowsList []*sqlmock.Rows) {
	if len(tables) != len(counts) || len(tables) != len(rowsList) {
		panic("tables, counts, and rowsList must have the same length")
	}

	for i, table := range tables {
		// Setup count expectation
		h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
			WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(counts[i]))

		// Setup rows expectation
		h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE", table)).
			WillReturnRows(rowsList[i])
	}
}

// buildTextSearchWhereClause builds a WHERE clause for text search
func (h *SearchTestHelper) buildTextSearchWhereClause(fields []string, term string) string {
	var conditions []string
	for _, field := range fields {
		conditions = append(conditions, fmt.Sprintf("%s LIKE '%%%s%%'", field, term))
	}
	return "(" + strings.Join(conditions, " OR ") + ")"
}

// ExpectSearchWithPagination sets up expectations for a search operation with pagination
func (h *SearchTestHelper) ExpectSearchWithPagination(table, query string, pagination queryutil.Pagination, count int64, rows *sqlmock.Rows) {
	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Setup rows expectation with LIMIT and OFFSET
	limitClause := fmt.Sprintf("LIMIT %d", pagination.PageSize)
	if pagination.Start > 0 {
		limitClause = fmt.Sprintf("LIMIT %d OFFSET %d", pagination.PageSize, pagination.Start)
	}

	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE .+ %s", table, limitClause)).
		WillReturnRows(rows)
}

// ExpectSearchWithSorting sets up expectations for a search operation with sorting
func (h *SearchTestHelper) ExpectSearchWithSorting(table, query string, sorts []queryutil.Sort, count int64, rows *sqlmock.Rows) {
	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE", table)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Build ORDER BY clause
	var orderClauses []string
	for _, sort := range sorts {
		direction := "ASC"
		if sort.Order == queryutil.OrderDesc {
			direction = "DESC"
		}
		orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sort.Field, direction))
	}
	orderByClause := "ORDER BY " + strings.Join(orderClauses, ", ")

	// Setup rows expectation with ORDER BY
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE .+ %s", table, orderByClause)).
		WillReturnRows(rows)
}

// ExpectSearchWithGlobalSearch sets up expectations for a search with global search config
func (h *SearchTestHelper) ExpectSearchWithGlobalSearch(table string, searchTerm string,
	searchConfig *queryutil.GlobalSearchConfig, count int64, rows *sqlmock.Rows) {

	if searchConfig == nil || len(searchConfig.SearchableColumns) == 0 {
		// Fall back to regular search if no global search config
		h.ExpectSearch(table, searchTerm, count, rows)
		return
	}

	// Build WHERE clause for global search
	var conditions []string
	for _, column := range searchConfig.SearchableColumns {
		conditions = append(conditions, fmt.Sprintf("%s LIKE '%%%s%%'", column, searchTerm))
	}
	whereClause := "(" + strings.Join(conditions, " OR ") + ")"

	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	// Setup rows expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT (.+) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(rows)
}

// ExpectSearchCount sets up expectations for just the count part of a search
func (h *SearchTestHelper) ExpectSearchCount(table string, filters []queryutil.Filter, count int64) {
	whereClause := h.buildWhereClauseFromFilters(filters)

	// Setup count expectation
	h.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", table, whereClause)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))
}

// ExpectSearchRows sets up expectations for just the rows part of a search
func (h *SearchTestHelper) ExpectSearchRows(table string, filters []queryutil.Filter,
	sorts []queryutil.Sort, pagination queryutil.Pagination, rows *sqlmock.Rows) {

	whereClause := h.buildWhereClauseFromFilters(filters)

	// Build query with sorting and pagination
	query := fmt.Sprintf("SELECT (.+) FROM `%s` WHERE %s", table, whereClause)

	// Add ORDER BY if sorts are specified
	if len(sorts) > 0 {
		var orderClauses []string
		for _, sort := range sorts {
			direction := "ASC"
			if sort.Order == queryutil.OrderDesc {
				direction = "DESC"
			}
			orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sort.Field, direction))
		}
		query += " ORDER BY " + strings.Join(orderClauses, ", ")
	}

	// Add LIMIT and OFFSET if pagination is specified
	if pagination.PageSize > 0 {
		query += fmt.Sprintf(" LIMIT %d", pagination.PageSize)
		if pagination.Start > 0 {
			query += fmt.Sprintf(" OFFSET %d", pagination.Start)
		}
	}

	// Setup rows expectation
	h.tc.mock.ExpectQuery(query).WillReturnRows(rows)
}
