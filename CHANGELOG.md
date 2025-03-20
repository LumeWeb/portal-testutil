# Changelog

## 0.1.7 (2025-03-20)

### Added
- Added generic test utilities for improved type safety and developer experience:
  - `CreateAndRegisterService[T]`: Type-safe service creation and registration
  - `GetService[T]`: Type-safe service retrieval without type assertions
  - `RegisterModelWithRelationships[T]`: Automatic discovery and registration of related models
  - `TestSuite[T]`: Reusable test suite structure with typed service access
  - `ServiceInitializer` interface for flexible service initialization
  - `ExpectServiceTransaction` for simplified transaction expectation setup
  - `NewTestLogger` for standardized logger creation
- Enhanced documentation with extensive examples for all generic utilities

## 0.1.6 (2025-03-20)

### Added
- Added `ExpectCreate` and `ExpectCreateError` methods with automatic transaction handling:
  - Automatically handles GORM's transaction behavior (Begin/Commit/Rollback)
  - Provides a more semantic API that matches GORM's Create() method terminology
  - Supports both automatic and manual transaction handling modes
  - Optional boolean parameter to disable automatic transaction handling
  - Comprehensive test coverage for both modes
  - Complete documentation with examples for various use cases
- Added transaction-based Create methods:
  - Added `Create(id)` method to TransactionExpectationBuilder as a semantic wrapper for Insert
  - Added `CreateError(err)` method to TransactionExpectationBuilder for error handling

### Fixed
- Fixed SQL pattern matching for GORM's Create operations by using ExpectExec instead of ExpectQuery

## 0.1.5 (2025-03-20)

### Added
- Added transaction table resolution for registered models:
  - Solved "Table not set" errors when using registered models in transactions
  - Added automatic table name resolution for models with custom TableName() methods
  - Enhanced transaction helper with GORM callbacks to ensure correct table resolution
  - Added support for all transaction operations (Create, Update, Delete, Query)
  - Comprehensive test suite for transaction table resolution

### Fixed
- Fixed "Table not set" error in transaction operations with registered models
- Fixed table name resolution issue with custom TableName() methods in transactions

## 0.1.4 (2025-03-20)

### Added
- Added new specialized methods for handling complex GORM query patterns:
  - `HandleStandardFirstRows()`: For reliable testing of basic First() queries
  - `HandleDeletedNotNullRows()`: For testing soft-deleted record retrieval
  - `HandleDeletedAtRows()`: For testing queries with explicit deleted_at conditions
- Added `WithDeletedAt()` method for better soft delete handling in tests
- Added `SkipVerification()` method to bypass expectation verification when needed

### Fixed
- Fixed SQL pattern matching for GORM's complex query patterns
- Improved testing of First() queries with automatic detection of First()-like patterns
- Fixed pattern matching for complex WHERE clauses with parentheses
- Enhanced handling of soft delete conditions in GORM queries
- Improved compatibility with non-soft-delete models
- Better handling of argument matching for various GORM query patterns

### Changed
- Improved diagnostic output for SQL pattern matching failures
- Added more detailed pattern logging when debug mode is enabled
- Enhanced detection of First()-like queries without requiring explicit First() calls

## 0.1.3 (2025-03-19)

### Added
- Enhanced Count Query Support:
  - Updated `ExpectCount()` to use "count(*)" column name by default for better ORM compatibility
  - Added `WithColumnName()` option for customizing the column name in count results
  - Improved compatibility with various ORM count query patterns
  - Added automatic type conversion for int/int64 count arguments
  - Complete unit tests for different count scenarios
  - Added comprehensive documentation for the enhanced count functionality

### Fixed
- Fixed the scan error issue with count queries where "count(*)" was expected but "count" was provided

## 0.1.2 (2025-03-19)

### Added
- BuildRows functionality for simplified row creation:
  - Added `BuildRows()` method for creating rows from a map
  - Added `BuildRowsFrom()` method for creating rows from structs or models
  - Support for GORM models with automatic field mapping
  - Support for embedded structs and custom column name tags
  - Flexible API that works with single objects or slices
- Improved test scenario step naming (removed colons from test names)
- Complete unit tests for new BuildRows functionality

### Changed
- Updated code to use modern Go syntax with `any` instead of `interface{}`
- Enhanced GoDoc documentation with section headers and comprehensive examples
- More robust field name handling for database column mapping

## 0.1.1 (2025-03-19)

### Added
- Enhanced Count Query Support:
  - Added `CountExpectationBuilder` for flexible count expectations
  - Added `Where()` method to support filtered count queries
  - Added `ReturnError()` method to simulate database errors
  - Added `ReturnCount()` method for clearer API
- Proper integration with Portal's core testing utilities
- Foreign key and index query expectations for SQLite
- Improved RowsWrapper implementation for test data
- Additional search testing utilities
- Comprehensive documentation for library components
- Complete unit tests for new features

### Changed
- Refactored DBTestContext to properly extend core.TestContext
- Updated service mock registry to work with extended DBTestContext
- Improved architecture to follow Portal's core design patterns
- Simplified TestContext integration by embedding instead of wrapping
- Maintained backward compatibility with the original `ExpectCount(n)` syntax

### Fixed
- SQLite version query warnings in test output:
  - Added case-insensitive regex patterns for query matching
  - Added catch-all handler for unexpected query variations
  - Added support for PRAGMA version commands
  - Updated `VerifyExpectations()` to filter out SQLite-related false warnings
- Service interface compatibility with core.Service
- Row iteration in test data using improved wrapper