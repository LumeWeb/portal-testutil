# Changelog

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