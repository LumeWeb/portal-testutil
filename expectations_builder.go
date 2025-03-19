// Package testutil provides utilities for testing service components
package testutil

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/DATA-DOG/go-sqlmock"
	"go.lumeweb.com/queryutil"
	"gorm.io/gorm"
)

// Helper function to add arguments to an expectation
func addArgsToExpectation(exp interface{}, args []interface{}) interface{} {
	// We need to handle both ExpectedExec and ExpectedQuery which both have WithArgs method
	// but don't share a common interface with it in sqlmock
	if len(args) == 0 {
		return exp // No args to add
	}

	// Convert arguments to driver.Value, which is what sqlmock.WithArgs expects
	driverArgs := make([]driver.Value, len(args))
	for i, arg := range args {
		driverArgs[i] = arg
	}

	switch e := exp.(type) {
	case *sqlmock.ExpectedExec:
		// Add all arguments at once to avoid conflicts with WithoutArgs
		return e.WithArgs(driverArgs...)
	case *sqlmock.ExpectedQuery:
		// Add all arguments at once to avoid conflicts with WithoutArgs
		return e.WithArgs(driverArgs...)
	default:
		// Return as is if it's not a supported type
		return exp
	}
}

// ExpectationsBuilder provides a fluent interface for building SQL expectations
// for a specific database table.
//
// This is the main builder used to set up expectations for common database operations
// like find, insert, update, delete, and search. It uses a builder pattern to provide
// a readable and expressive way to set up expectations.
//
// ExpectationsBuilder is typically created via DBTestContext.ForTable(), not directly.
//
// Example:
//
//	// Set up expectations for finding a user by ID
//	testCtx.ForTable("users").ExpectFind().ByID(1).ReturnRows(rows)
//
//	// Set up expectations for inserting a new user
//	testCtx.ForTable("users").ExpectInsert(1)
//
//	// Set up expectations for a transaction
//	testCtx.ForTable("users").ExpectTransaction().Insert(1).Commit()
type ExpectationsBuilder struct {
	tc    *DBTestContext // The test context that owns this builder
	table string         // The table name (with prefix if configured)
}

// NewExpectationsBuilder creates a new expectations builder
func NewExpectationsBuilder(tc *DBTestContext, table string) *ExpectationsBuilder {
	// Apply table prefix if configured
	if tc.config.TablePrefix != "" {
		// Only apply prefix if the table doesn't already have it
		if !strings.HasPrefix(table, tc.config.TablePrefix) {
			table = tc.config.TablePrefix + table
		}
	}

	return &ExpectationsBuilder{
		tc:    tc,
		table: table,
	}
}

// ExpectTransaction starts a transaction expectation
func (b *ExpectationsBuilder) ExpectTransaction() *TransactionExpectationBuilder {
	b.tc.mock.ExpectBegin()
	return &TransactionExpectationBuilder{
		builder: b,
	}
}

// ExpectFind creates a find expectation
func (b *ExpectationsBuilder) ExpectFind() *FindExpectationBuilder {
	return &FindExpectationBuilder{
		builder: b,
	}
}

// ExpectSearch creates a search expectation
func (b *ExpectationsBuilder) ExpectSearch(query string) *SearchExpectationBuilder {
	return &SearchExpectationBuilder{
		builder: b,
		query:   query,
	}
}

// ExpectInsert expects an insert operation and returns a row ID
func (b *ExpectationsBuilder) ExpectInsert(id uint) *ExpectationsBuilder {
	b.tc.mock.ExpectQuery("^INSERT INTO `" + b.table + "`").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	return b
}

// ExpectInsertError expects an insert operation that returns an error
func (b *ExpectationsBuilder) ExpectInsertError(err error) *ExpectationsBuilder {
	b.tc.mock.ExpectQuery("^INSERT INTO `" + b.table + "`").
		WillReturnError(err)
	return b
}

// ExpectUpdate expects an update operation
func (b *ExpectationsBuilder) ExpectUpdate() *ExpectationsBuilder {
	b.tc.mock.ExpectExec("^UPDATE `" + b.table + "` SET").
		WillReturnResult(sqlmock.NewResult(1, 1))
	return b
}

// ExpectUpdateError expects an update operation that returns an error
func (b *ExpectationsBuilder) ExpectUpdateError(err error) *ExpectationsBuilder {
	b.tc.mock.ExpectExec("^UPDATE `" + b.table + "` SET").
		WillReturnError(err)
	return b
}

// ExpectDelete expects a delete operation
func (b *ExpectationsBuilder) ExpectDelete() *ExpectationsBuilder {
	b.tc.mock.ExpectExec("^DELETE FROM `" + b.table + "`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	return b
}

// ExpectDeleteError expects a delete operation that returns an error
func (b *ExpectationsBuilder) ExpectDeleteError(err error) *ExpectationsBuilder {
	b.tc.mock.ExpectExec("^DELETE FROM `" + b.table + "`").
		WillReturnError(err)
	return b
}

// ExpectCount expects a count operation
func (b *ExpectationsBuilder) ExpectCount(count int64) *ExpectationsBuilder {
	b.tc.mock.ExpectQuery("^SELECT count\\(\\*\\) FROM `" + b.table + "`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
	return b
}

// TransactionExpectationBuilder builds expectations for a transaction
type TransactionExpectationBuilder struct {
	builder *ExpectationsBuilder
}

// Insert expects an insert within a transaction
func (t *TransactionExpectationBuilder) Insert(id uint) *TransactionExpectationBuilder {
	t.builder.ExpectInsert(id)
	return t
}

// InsertError expects an insert error within a transaction
func (t *TransactionExpectationBuilder) InsertError(err error) *TransactionExpectationBuilder {
	t.builder.ExpectInsertError(err)
	return t
}

// Update expects an update within a transaction
func (t *TransactionExpectationBuilder) Update() *TransactionExpectationBuilder {
	t.builder.ExpectUpdate()
	return t
}

// UpdateError expects an update error within a transaction
func (t *TransactionExpectationBuilder) UpdateError(err error) *TransactionExpectationBuilder {
	t.builder.ExpectUpdateError(err)
	return t
}

// Delete expects a delete within a transaction
func (t *TransactionExpectationBuilder) Delete() *TransactionExpectationBuilder {
	t.builder.ExpectDelete()
	return t
}

// Commit expects a transaction commit
func (t *TransactionExpectationBuilder) Commit() *ExpectationsBuilder {
	t.builder.tc.mock.ExpectCommit()
	return t.builder
}

// Rollback expects a transaction rollback
func (t *TransactionExpectationBuilder) Rollback() *ExpectationsBuilder {
	t.builder.tc.mock.ExpectRollback()
	return t.builder
}

// FindExpectationBuilder builds expectations for a find operation
type FindExpectationBuilder struct {
	builder *ExpectationsBuilder
	where   string
	args    []interface{}
}

// Where adds a where clause to the find expectation
func (f *FindExpectationBuilder) Where(where string, args ...interface{}) *FindExpectationBuilder {
	f.where = where
	f.args = args
	return f
}

// ByID adds an ID filter to the find expectation
func (f *FindExpectationBuilder) ByID(id uint) *FindExpectationBuilder {
	f.where = "`" + f.builder.table + "`.`id` = ?"
	f.args = []interface{}{id}
	return f
}

// ReturnRows sets the rows to return for the find expectation
func (f *FindExpectationBuilder) ReturnRows(rows *sqlmock.Rows) *ExpectationsBuilder {
	// Make the query pattern more flexible to match different variations
	// that GORM might generate (including SELECT * or SELECT specific columns)
	query := "SELECT (.+) FROM `" + f.builder.table + "`"
	if f.where != "" {
		// Replace the field name with a more flexible pattern that can handle backticks
		where := strings.ReplaceAll(f.where, "`"+f.builder.table+"`.`", ".*`")
		query += " WHERE " + where
	}

	exp := f.builder.tc.mock.ExpectQuery(query)

	// Add arguments all at once
	exp = addArgsToExpectation(exp, f.args).(*sqlmock.ExpectedQuery)

	exp.WillReturnRows(rows)
	return f.builder
}

// ReturnError sets an error to return for the find expectation
func (f *FindExpectationBuilder) ReturnError(err error) *ExpectationsBuilder {
	// Make the query pattern more flexible to match different variations
	// that GORM might generate (including SELECT * or SELECT specific columns)
	query := "SELECT (.+) FROM `" + f.builder.table + "`"
	if f.where != "" {
		// Replace the field name with a more flexible pattern that can handle backticks
		where := strings.ReplaceAll(f.where, "`"+f.builder.table+"`.`", ".*`")
		query += " WHERE " + where
	}

	exp := f.builder.tc.mock.ExpectQuery(query)

	// Add arguments all at once
	exp = addArgsToExpectation(exp, f.args).(*sqlmock.ExpectedQuery)

	exp.WillReturnError(err)
	return f.builder
}

// NotFound returns a not found error for the find expectation
func (f *FindExpectationBuilder) NotFound() *ExpectationsBuilder {
	return f.ReturnError(gorm.ErrRecordNotFound)
}

// GenericExpectationBuilder provides a more generic interface for building expectations
type GenericExpectationBuilder struct {
	tc *DBTestContext
}

// NewGenericExpectationBuilder creates a new generic expectation builder
func NewGenericExpectationBuilder(tc *DBTestContext) *GenericExpectationBuilder {
	return &GenericExpectationBuilder{
		tc: tc,
	}
}

// Transaction starts a transaction expectation
func (g *GenericExpectationBuilder) Transaction() *GenericExpectationBuilder {
	g.tc.mock.ExpectBegin()
	return g
}

// Query expects a custom SQL query
func (g *GenericExpectationBuilder) Query(query string) *QueryExpectationBuilder {
	return &QueryExpectationBuilder{
		builder: g,
		query:   query,
	}
}

// Exec expects a custom SQL execution
func (g *GenericExpectationBuilder) Exec(query string) *ExecExpectationBuilder {
	return &ExecExpectationBuilder{
		builder: g,
		query:   query,
	}
}

// Commit expects a transaction commit
func (g *GenericExpectationBuilder) Commit() *GenericExpectationBuilder {
	g.tc.mock.ExpectCommit()
	return g
}

// Rollback expects a transaction rollback
func (g *GenericExpectationBuilder) Rollback() *GenericExpectationBuilder {
	g.tc.mock.ExpectRollback()
	return g
}

// QueryExpectationBuilder builds expectations for a query
type QueryExpectationBuilder struct {
	builder *GenericExpectationBuilder
	query   string
	args    []interface{}
}

// WithArgs adds arguments to the query expectation
func (q *QueryExpectationBuilder) WithArgs(args ...interface{}) *QueryExpectationBuilder {
	q.args = args
	return q
}

// ReturnRows sets the rows to return for the query expectation
func (q *QueryExpectationBuilder) ReturnRows(rows *sqlmock.Rows) *GenericExpectationBuilder {
	exp := q.builder.tc.mock.ExpectQuery(q.query)

	// Add arguments one by one
	exp = addArgsToExpectation(exp, q.args).(*sqlmock.ExpectedQuery)

	exp.WillReturnRows(rows)
	return q.builder
}

// ReturnError sets an error to return for the query expectation
func (q *QueryExpectationBuilder) ReturnError(err error) *GenericExpectationBuilder {
	exp := q.builder.tc.mock.ExpectQuery(q.query)

	// Add arguments one by one
	exp = addArgsToExpectation(exp, q.args).(*sqlmock.ExpectedQuery)

	exp.WillReturnError(err)
	return q.builder
}

// ExecExpectationBuilder builds expectations for an exec operation
type ExecExpectationBuilder struct {
	builder *GenericExpectationBuilder
	query   string
	args    []interface{}
}

// WithArgs adds arguments to the exec expectation
func (e *ExecExpectationBuilder) WithArgs(args ...interface{}) *ExecExpectationBuilder {
	e.args = args
	return e
}

// ReturnResult sets the result to return for the exec expectation
func (e *ExecExpectationBuilder) ReturnResult(rowsAffected int64) *GenericExpectationBuilder {
	exp := e.builder.tc.mock.ExpectExec(e.query)

	// Add arguments one by one
	exp = addArgsToExpectation(exp, e.args).(*sqlmock.ExpectedExec)

	exp.WillReturnResult(sqlmock.NewResult(1, rowsAffected))
	return e.builder
}

// ReturnError sets an error to return for the exec expectation
func (e *ExecExpectationBuilder) ReturnError(err error) *GenericExpectationBuilder {
	exp := e.builder.tc.mock.ExpectExec(e.query)

	// Add arguments one by one
	exp = addArgsToExpectation(exp, e.args).(*sqlmock.ExpectedExec)

	exp.WillReturnError(err)
	return e.builder
}

// SearchExpectationBuilder builds expectations for a search operation
type SearchExpectationBuilder struct {
	builder    *ExpectationsBuilder
	query      string
	fields     []string
	filters    []queryutil.Filter
	sorts      []queryutil.Sort
	pagination queryutil.Pagination
}

// WithFields specifies which fields to search in
func (s *SearchExpectationBuilder) WithFields(fields ...string) *SearchExpectationBuilder {
	s.fields = fields
	return s
}

// WithFilters adds filters to the search expectation
func (s *SearchExpectationBuilder) WithFilters(filters ...queryutil.Filter) *SearchExpectationBuilder {
	s.filters = filters
	return s
}

// WithSorts adds sorting to the search expectation
func (s *SearchExpectationBuilder) WithSorts(sorts ...queryutil.Sort) *SearchExpectationBuilder {
	s.sorts = sorts
	return s
}

// WithPagination adds pagination to the search expectation
func (s *SearchExpectationBuilder) WithPagination(pagination queryutil.Pagination) *SearchExpectationBuilder {
	s.pagination = pagination
	return s
}

// ReturnCount sets the count to return for the search expectation
func (s *SearchExpectationBuilder) ReturnCount(count int64) *SearchExpectationBuilder {
	// Build the WHERE clause for the search
	whereClause := s.buildWhereClause()

	// Setup count expectation
	s.builder.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", s.builder.table, whereClause)).
		WillReturnRows(sqlmock.NewRows([]string{"count(*)"}).AddRow(count))

	return s
}

// ReturnRows sets the rows to return for the search expectation
func (s *SearchExpectationBuilder) ReturnRows(rows *sqlmock.Rows) *ExpectationsBuilder {
	// Build the WHERE clause for the search
	whereClause := s.buildWhereClause()

	// Build the query with optional ORDER BY and LIMIT clauses
	query := fmt.Sprintf("SELECT (.+) FROM `%s` WHERE %s", s.builder.table, whereClause)

	// Add ORDER BY if sorts are specified
	if len(s.sorts) > 0 {
		var orderClauses []string
		for _, sort := range s.sorts {
			direction := "ASC"
			if sort.Order == queryutil.OrderDesc {
				direction = "DESC"
			}
			orderClauses = append(orderClauses, fmt.Sprintf("%s %s", sort.Field, direction))
		}
		query += " ORDER BY " + strings.Join(orderClauses, ", ")
	}

	// Add LIMIT if pagination is specified
	if s.pagination.PageSize > 0 {
		limitClause := fmt.Sprintf("LIMIT %d", s.pagination.PageSize)
		if s.pagination.Start > 0 {
			limitClause = fmt.Sprintf("LIMIT %d OFFSET %d", s.pagination.PageSize, s.pagination.Start)
		}
		query += " " + limitClause
	}

	// Setup rows expectation
	s.builder.tc.mock.ExpectQuery(query).
		WillReturnRows(rows)

	return s.builder
}

// ReturnError sets an error to return for the search expectation
func (s *SearchExpectationBuilder) ReturnError(err error) *ExpectationsBuilder {
	// Build the WHERE clause for the search
	whereClause := s.buildWhereClause()

	// Setup count expectation with error
	s.builder.tc.mock.ExpectQuery(fmt.Sprintf("SELECT count\\(\\*\\) FROM `%s` WHERE %s", s.builder.table, whereClause)).
		WillReturnError(err)

	return s.builder
}

// buildWhereClause builds the WHERE clause for the search
func (s *SearchExpectationBuilder) buildWhereClause() string {
	var conditions []string

	// Add text search condition if query and fields are specified
	if s.query != "" && len(s.fields) > 0 {
		var fieldConditions []string
		for _, field := range s.fields {
			fieldConditions = append(fieldConditions, fmt.Sprintf("%s LIKE '%%%s%%'", field, s.query))
		}
		conditions = append(conditions, "("+strings.Join(fieldConditions, " OR ")+")")
	}

	// Add filter conditions
	for _, filter := range s.filters {
		var condition string
		switch filter.Operator {
		case queryutil.OperatorEquals:
			condition = fmt.Sprintf("%s = '%v'", filter.Field, filter.Value)
		case queryutil.OperatorNotEquals:
			condition = fmt.Sprintf("%s != '%v'", filter.Field, filter.Value)
		case queryutil.OperatorContains:
			condition = fmt.Sprintf("%s LIKE '%%%v%%'", filter.Field, filter.Value)
		case queryutil.OperatorGTE:
			condition = fmt.Sprintf("%s >= %v", filter.Field, filter.Value)
		case queryutil.OperatorLTE:
			condition = fmt.Sprintf("%s <= %v", filter.Field, filter.Value)
		}
		conditions = append(conditions, condition)
	}

	// If no conditions, use a default condition that matches everything
	if len(conditions) == 0 {
		return "1=1"
	}

	return strings.Join(conditions, " AND ")
}
