# Changelog

## 0.2.22 (2025-03-22)

### Fixed
- Fixed bug in model-to-SQL row conversion that caused relationship ID fields to be missing:
  - Ensures foreign key fields like ReporterID and SubjectID are preserved in SQL rows
  - Resolves validation errors like "subject is required" or "unauthorized access" in tests
  - Improved overall reliability of relationship handling
  - Enhanced performance with optimized field detection

## 0.2.21 (2025-03-22)

### Fixed
- Fixed SQL pattern matching for Where + WithDeletedAt + First combination:
  - Expanded the fix from 0.2.19 to handle all Where conditions (not just ByID)
  - Made the pattern handling more generic to work with any WHERE clause
  - Fixed issue where general WHERE conditions weren't properly matched when combined with WithDeletedAt and First
  - Ensures SQL pattern correctly accounts for the deleted_at IS NULL condition position in all queries
  - Added comprehensive test cases for both success and error cases with custom WHERE conditions
  - Eliminates the need for manually writing Raw SQL patterns for these common combinations

## 0.2.20 (2025-03-22)

### Fixed
- Fixed inconsistency between ReturnRows and ReturnError for the WithDeletedAt + ByID + First combination:
  - Added the same exact SQL pattern handling to ReturnError that was added to ReturnRows in 0.2.19
  - Ensured consistent behavior between success and error cases with the same query pattern
  - Added comprehensive test case for the error path to verify the fix works correctly
  - Fixed potential regression when testing error cases with this specific combination of methods

## 0.2.19 (2025-03-22)

### Fixed
- Fixed SQL pattern matching issue with WithDeletedAt + ByID + First combination:
  - Added specific pattern handling for the combination of these three methods
  - Properly matches GORM's SQL query when using First() with a primary key on a soft delete model
  - Fixed issue where deleted_at IS NULL condition was inserted between ID condition and ORDER BY clause
  - Ensures SQL pattern correctly accounts for the exact position of the deleted_at IS NULL condition
  - Added comprehensive test case to verify the fix works with this specific combination
  - Eliminates the need for using raw SQL expectations for these common query patterns

## 0.2.18 (2025-03-21)

### Added
- Added SerializeMapFields utility function to handle map fields in GORM models:
  - Provides a dedicated helper to convert map fields to JSON strings to avoid the "unsupported data type: &map[]" error
  - Works with both single models and slices of models
  - Handles different map field types including direct maps and slices of maps
  - Processes nested structures while preserving original model data
  - Returns a copy of the model with map fields converted to a format compatible with SQL libraries
  - Includes comprehensive tests and examples showing usage patterns

### Fixed
- Added documentation and examples for working with map fields in GORM models to avoid the "unsupported data type: &map[]" error:
  - Demonstrated serializing maps to JSON strings in model fields
  - Created comprehensive example in test files
  - Clarified that go-sqlmock has limitations with map fields in test environments
  - Provided patterns to properly store and retrieve map data in tests

## 0.2.17 (2025-03-21)

### Fixed
- Fixed WHERE clause compatibility with ReturnModelsWithPreload():
  - Fixed issue where Where() conditions weren't properly matched when using ReturnModelsWithPreload()
  - Enhanced SQL pattern matching to properly handle WHERE clauses with relationship data
  - Added support for Where() conditions with parameterized queries (using ?)
  - Works with both ExpectFind() and ExpectSearch() builders
  - Comprehensive test cases added for both WHERE clause and search scenarios

## 0.2.16 (2025-03-21)

### Added
- Added relationship preloading support for test data with ReturnModelsWithPreload():
  - Powerful new method that automatically populates GORM relationship fields
  - Eliminates the need for explicit Preload() calls in test queries
  - Removes the requirement for multiple mock expectations (one per relationship)
  - Works with all GORM relationship types (has-many, has-one, belongs-to)
  - Supports nested relationships and complex model structures
  - Includes comprehensive documentation and examples
  - Compatible with ExpectFind(), ExpectSearch(), and other query builders

### Fixed
- Fixed relationship handling limitation where relationship fields were empty in test results:
  - Before: Your test data had populated relationship fields, but they came back empty in results
  - Now: Relationship fields are automatically populated in query results
  - Eliminates the need for separate mock expectations for relationship queries
  - Makes tests with relationship models much simpler and more intuitive

## 0.2.15 (2025-03-21)

### Fixed
- Fixed SQL pattern matching for COUNT queries with soft delete tables:
  - Added intelligence to predict when GORM will add soft delete clauses
  - Implemented flexible regex pattern matching as fallback for both Table() and Model() approaches
  - Fixed regression with queries on soft delete tables:
    - Model(&model).Count() adds the deleted_at IS NULL clause
    - Table("table").Count() doesn't add the deleted_at clause
  - Both approaches now work with a single ExpectCount() setup

## 0.2.14 (2025-03-21)

### Fixed
- Fixed SQL pattern matching for soft delete tables:
  - Added missing WHERE keyword in SQL patterns for soft delete conditions
  - This ensures correct matching of actual SQL queries GORM generates

- Fixed `SearchExpectationBuilder` to use parameterized queries:
  - Updated buildWhereClause to use ? placeholders instead of string literals
  - Modified arguments handling to work with GORM's parameter usage
  - Now properly works with LIKE conditions in GORM queries

- Enhanced all `ReturnModels()` methods to internally use `BuildRowsWithRelations()` instead of `BuildRowsFrom()`:
  - Fixed the high-level API to work properly with GORM relationship models 
  - Updated `FindExpectationBuilder.ReturnModels()` to handle relationship fields
  - Updated `SearchExpectationBuilder.ReturnModels()` to handle relationship fields
  - Updated `QueryExpectationBuilder.ReturnModels()` to handle relationship fields
  - This eliminates the "unsupported data type: &map[]" error when using ReturnModels() with relationships

### Added
- Added comprehensive test examples for relationship models:
  - Direct querying tests with ExpectFind()
  - Search testing with ExpectSearch()
  - Custom SQL queries with Expect().Query()
  - Filter tests with ExpectFind().Where()

## 0.2.13 (2025-03-21)

### Added
- Added `BuildRowsWithRelations()` method to properly handle GORM relationship fields in tests:
  - Solves the "unsupported data type: &map[]" error when testing models with relationships
  - Uses JSON serialization for relationship fields to maintain their structure
  - Supports belongs-to, has-one, and has-many relationships
  - Maintains proper foreign key references
  - Works with nested relationship structures
  - Includes an `SQLRelationshipScanner` type for advanced usage
  - Comprehensive documentation and examples for relationship handling

## 0.2.12 (2025-03-21)

### Added
- Added `WithArgs()` method to provide explicit argument matching for SQL queries:
  - Works with `ExpectCount()`, `ExpectFind()` and other query builders
  - Allows exact argument matching for parameterized SQL queries with WHERE clauses
  - Provides better testing of filter conditions and search parameters
  - Great for testing complex query conditions with multiple parameters
  
  **Important usage note:** You MUST call `Where()` before `WithArgs()`. The `WithArgs()` method only specifies the arguments to match and does not create the SQL pattern. Correct usage: `.Where("status = ?").WithArgs("active")`

## 0.2.11 (2025-03-21)

### Added
- Added `ReturnModels()` method to `ExpectFind()` to simplify returning model slices:
  - Now you can directly pass model slices to `ExpectFind().ReturnModels(myModels)` 
  - No need to manually convert models to rows using `BuildRowsFrom()`
  - Added similar methods to `ExpectSearch()` and custom `Query()` builders
  - Makes API more consistent and user-friendly

## 0.2.10 (2025-03-21)

### Added
- Added `MockWithDefaults()` method to automatically handle GORM's connection validation queries
- Enhanced documentation to warn about `SELECT 1` validation queries when using direct mock access

## 0.2.9 (2025-03-21)

### Fixed
- Improved SQL pattern matching documentation for count queries:
  - Added comprehensive godocs about model registration for soft delete handling
  - Updated documentation to explain the removal of `^` anchor for better GORM compatibility
- Implemented previously skipped `TestBuildRowsFrom_UnsupportedType` test

## 0.2.8 (2025-03-21)

### Added
- Added `Mock()` method as an intuitive alias for `Raw()` to provide direct access to the underlying sqlmock

### Fixed
- Fixed SQL pattern matching in `ExpectCount()` to correctly work with GORM's generated queries

## 0.2.7 (2025-03-21)

### Added
- Enhanced `BuildRowsFrom` to properly handle complex GORM models:
  - Support for models with `gorm.Model` embedding and soft delete functionality
  - Support for models with relationship fields (both regular and pointer types)
  - Smart handling of has-many relationships
  - Enhanced documentation with examples

### Fixed
- Fixed scanning errors when working with `gorm.DeletedAt` fields
- Fixed panic when working with models containing relationship fields 
- Improved support for nil pointer relationships in test models

## 0.2.6 (2025-03-21)

### Added
- Automatic table resolution for all GORM operations in test environments
- Support for complex model relationships across different packages

### Fixed
- Fixed "Table not set" error when using models with both relationships and lifecycle hooks
- Fixed transaction handling for complex model structures
- Resolved edge cases with validation hooks in transactional contexts

### Improved
- Enhanced documentation with better explanations of table resolution
- Added comprehensive test coverage for different model structures
- Reduced need for manual table name handling in tests

## 0.2.5 (2025-03-20)

### Added
- Integrated `github.com/gertd/go-pluralize` library for more accurate pluralization of table names
- Enhanced table name resolution in transactions with specialized handling for complex models having both relationships and validation methods
- New test case `TestModelsWithRelationshipsAndValidation` to verify fix for models with both relationships and hooks

### Fixed
- Issue where models with both relationships AND validation hooks would lose table information in transactions
- Improved the transaction helper to register additional callbacks that ensure proper table resolution
- Enhanced SQL pattern matching for First() method queries with table-qualified columns

### Changed
- Replaced custom pluralization logic with the more robust `pluralize` library
- Updated documentation in `HandleStandardFirstRows` method to better explain when and how to use it
- Added comprehensive troubleshooting section to README.md for common issues like "Table not set" errors
- Updated `TestTransactionMultipleOperations` to use table helpers instead of raw SQL

### Improved
- Added detailed comments throughout the codebase explaining complex table resolution logic
- Enhanced validation for models with both hooks and relationships to prevent "Table not set" errors
- Better handling of models with complex validation methods that use map operations

## 0.2.4 (2025-03-20)

### Fixed
- Recovered from code corruption in v0.2.3 release:
  - Fixed serious syntax errors found in the v0.2.3 release that caused build failures
  - Completely rebuilt the corrupted transaction_test_helpers.go file
  - Restored proper hook and relationship detection functionality
  - Ensured comprehensive test coverage for all code paths
  - Properly validated all changes with robust test suite

## 0.2.3 (2025-03-20) [CORRUPTED - DO NOT USE]

**WARNING: This release contains syntax errors causing build failures. Use v0.2.4 instead.**

### Fixed
- Fixed table name resolution for models with both relationships AND lifecycle hooks in transactions:
  - Added detection and special handling for models with both hooks and relationships
  - Enhanced relationship detection to identify both explicit GORM relationship tags and inferred relationships
  - Implemented clean instance creation to avoid hook interference with table name resolution
  - Improved handling in `tryGetTableName` and `ensureTableSet` functions for models with hooks
  - Added comprehensive test cases for all model types: simple, relationships-only, hooks-only, and the problematic combined case
- Added test verification for hook execution in transaction context
- Added example code and documentation in README for the fix

## 0.2.2 (2025-03-20)

### Fixed
- Fixed issue with table name resolution for complex models with relationships in transactions:
  - Added enhanced table name resolution for models with relationships
  - Added type name matching as a fallback when direct type comparison fails
  - Added GORM struct tag extraction for table information
  - Improved handling of models that undergo internal transformation by GORM
  - Added comprehensive tests for CRUD operations with complex models
- Eliminated the need for the workaround of manually setting table names in transactions for complex models
- Added detailed documentation in godocs and README about the enhanced table name resolution for complex models

## 0.2.1 (2025-03-20)

### Fixed
- Fixed regression in v0.2.0 where table name resolution still failed in certain transaction scenarios:
  - Enhanced table name resolution for map values in transactions
  - Fixed "Table not set" error when using RegisterModelWithRelationships with custom TableName models
  - Added support for extracting table name from GORM's Statement.Dest field
  - Improved transaction wrapper to handle model-less operations
- Eliminated GORM callback warnings by implementing precise callback tracking:
  - Added unique session ID for each transaction helper instance
  - Implemented tracking of registered callbacks by name and active state
  - Added callback removal that only removes callbacks actually registered
  - Prevented duplicate callback warnings with unique callback naming scheme
  - Added comprehensive test suite for callback tracking functionality
- Added extensive documentation for all enhancements in README, godocs, and CHANGELOG

## 0.2.0 (2025-03-20)

### Added
- Support for GORM v1.25+ RETURNING clause in insert operations
- Comprehensive table name resolution for transactions with improved handling of:
  - Value receiver TableName() methods
  - Pointer receiver TableName() methods
  - Nil pointer models
  - Slices of models
  - Direct struct values
- Improved documentation for transaction helpers and table resolution

### Fixed
- Fixed critical bug in table name resolution with custom TableName() methods
- Fixed inconsistency between ExpectInsert and ExpectCreate APIs regarding transaction handling
- Fixed "Table not set" error when using any type of model in transactions
- Eliminated duplicate callback warnings by adding proper callback deduplication

### Changed
- ExpectInsert now automatically handles transactions like ExpectCreate
- ExpectCreate is now a semantic alias for ExpectInsert
- Both ExpectInsert and ExpectCreate support automatic transaction handling with optional boolean parameter
- Enhanced godocs for key functions with detailed explanations and examples

### Breaking Changes
- **ExpectInsert** now handles transactions automatically by default (previously it did not)
  - If you rely on the previous behavior where ExpectInsert did not handle transactions, you must now explicitly pass `false` as a second parameter: `ExpectInsert(id, false)`
  - This change was made to ensure consistency with ExpectCreate and remove API inconsistencies

## 0.1.8 (2025-03-20)

### Added
- Enhanced transaction support for GORM operations:
  - Added support for direct GORM transactions (`db.Transaction()`) with proper table resolution
  - Table names are now correctly resolved in both transaction helpers and direct GORM transactions
  - GORM callbacks integrated into the main DB instance for seamless table resolution
  - Added comprehensive test suite for direct transaction operations
  - Updated documentation with examples for both transaction helper and direct transaction patterns

### Fixed
- Fixed "Table not set" error in direct GORM transactions when using registered models
- Fixed table name resolution within nested transaction operations

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