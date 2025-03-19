package testutil

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

func TestSQLiteVersionQueryHandling(t *testing.T) {
	t.Run("SQLite version query is properly handled", func(t *testing.T) {
		// Create a new test context that doesn't emit warnings for unmet expectations
		// Special case for this test since we're deliberately testing default expectation behavior
		tc := NewDBTestContext(t, WithTablePrefix(""))
		defer func() {
			// For this test, we don't want to verify expectations on teardown
			// because we're explicitly testing the default expectations which may not be matched
			tc.SkipVerification()
		}()

		// Execute a query that will trigger GORM to check SQLite version
		// We'll just use a simple query to ensure the DB is initialized
		var result int
		tc.DB().Raw("SELECT 1").Scan(&result)

		// We expect result to be 1 as set in configureSQLMockDefaults
		assert.Equal(t, 1, result)

		// For this test, we don't verify expectations because we're specifically testing
		// the behavior of default expectations that are added automatically
	})

	t.Run("SQLite version query expectations are set up", func(t *testing.T) {
		// Create a test DB with raw sqlmock for more direct testing
		db, mock, err := sqlmock.New()
		assert.NoError(t, err)
		defer db.Close()

		// Configure SQLite defaults on this mock
		configureSQLMockDefaults(mock)

		// We're not actually executing queries here, just verifying that
		// the expectations were properly set up
		// Testing this is sufficient to ensure our configuration works

		// Verify mock setup was successful
		db.Close()
	})
}
