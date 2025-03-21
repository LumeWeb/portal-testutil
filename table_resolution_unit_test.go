package testutil

import (
	"testing"
	"reflect"
	"gorm.io/gorm"
)

// SimpleModelForTest is a test model with a TableName method
type SimpleModelForTest struct {
	ID   uint
	Name string
}

// TableName implements schema.Tabler interface
func (SimpleModelForTest) TableName() string {
	return "simple_test_models"
}

// ModelWithHooksForTest is a test model with hooks
type ModelWithHooksForTest struct {
	ID   uint
	Name string
}

// TableName implements schema.Tabler interface
func (ModelWithHooksForTest) TableName() string {
	return "hook_test_models"
}

// BeforeCreate adds a hook
func (m *ModelWithHooksForTest) BeforeCreate(tx *gorm.DB) error {
	// Add a map operation to trigger the bug
	validNames := map[string]bool{
		"test": true,
		"demo": true,
	}
	
	_ = validNames[m.Name]
	return nil
}

// ExternalPackageModel simulates a model from another package
type ExternalPackageModel struct {
	ID   uint
	Name string
}

// TableName returns a table name
func (ExternalPackageModel) TableName() string {
	return "external_package_models"
}

// RelatedModelForTest has a relationship to another model
type RelatedModelForTest struct {
	ID         uint
	ExternalID uint
	External   ExternalPackageModel `gorm:"foreignKey:ExternalID"`
	Name       string
}

// TableName implements schema.Tabler interface
func (RelatedModelForTest) TableName() string {
	return "related_test_models"
}

// BeforeCreate adds a hook with map operations
func (m *RelatedModelForTest) BeforeCreate(tx *gorm.DB) error {
	// Add a map operation to trigger the bug
	validNames := map[string]bool{
		"test": true,
		"demo": true,
	}
	
	_ = validNames[m.Name]
	return nil
}

// Test_ensureTableIsSet directly tests the ensureTableIsSet function
func Test_ensureTableIsSet(t *testing.T) {
	tests := []struct {
		name      string
		setupDB   func() *gorm.DB
		want      string
	}{
		{
			name: "SimpleModel",
			setupDB: func() *gorm.DB {
				model := &SimpleModelForTest{Name: "Test"}
				db := &gorm.DB{Config: &gorm.Config{}}
				db.Statement = &gorm.Statement{DB: db}
				db.Statement.Model = model
				db.Statement.ReflectValue = reflect.ValueOf(model)
				return db
			},
			want: "simple_test_models",
		},
		{
			name: "ModelWithHooks",
			setupDB: func() *gorm.DB {
				model := &ModelWithHooksForTest{Name: "Test"}
				db := &gorm.DB{Config: &gorm.Config{}}
				db.Statement = &gorm.Statement{DB: db}
				db.Statement.Model = model
				db.Statement.ReflectValue = reflect.ValueOf(model)
				return db
			},
			want: "hook_test_models",
		},
		{
			name: "RelatedModel",
			setupDB: func() *gorm.DB {
				model := &RelatedModelForTest{
					ExternalID: 1,
					Name:       "Test",
				}
				db := &gorm.DB{Config: &gorm.Config{}}
				db.Statement = &gorm.Statement{DB: db}
				db.Statement.Model = model
				db.Statement.ReflectValue = reflect.ValueOf(model)
				return db
			},
			want: "related_test_models",
		},
		{
			name: "ReflectValueOnly",
			setupDB: func() *gorm.DB {
				model := &SimpleModelForTest{Name: "Test"}
				db := &gorm.DB{Config: &gorm.Config{}}
				db.Statement = &gorm.Statement{DB: db}
				// No Model set, only ReflectValue
				db.Statement.ReflectValue = reflect.ValueOf(model)
				return db
			},
			want: "simple_test_models",
		},
		{
			name: "DestOnly",
			setupDB: func() *gorm.DB {
				model := &SimpleModelForTest{Name: "Test"}
				db := &gorm.DB{Config: &gorm.Config{}}
				db.Statement = &gorm.Statement{DB: db}
				// Set only Dest
				db.Statement.Dest = model
				return db
			},
			want: "simple_test_models",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := tt.setupDB()
			// Call the function being tested
			ensureTableIsSet(db)
			if db.Statement.Table != tt.want {
				t.Errorf("ensureTableIsSet() got table = %v, want %v", db.Statement.Table, tt.want)
			}
		})
	}
}

// Test_tryGetTableName tests the TransactionTestHelper.tryGetTableName function
func Test_tryGetTableName(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	
	// Create a transaction helper to access tryGetTableName
	th := NewTransactionTestHelper(tc)
	
	tests := []struct {
		name  string
		model interface{}
		want  string
	}{
		{
			name:  "SimpleModel",
			model: &SimpleModelForTest{Name: "Test"},
			want:  "simple_test_models",
		},
		{
			name:  "ModelWithHooks",
			model: &ModelWithHooksForTest{Name: "Test"},
			want:  "hook_test_models",
		},
		{
			name:  "RelatedModel",
			model: &RelatedModelForTest{ExternalID: 1, Name: "Test"},
			want:  "related_test_models",
		},
		{
			name:  "NilModel",
			model: nil,
			want:  "",
		},
		{
			name:  "NonPointerModel",
			model: SimpleModelForTest{Name: "Test"},
			want:  "simple_test_models",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.tryGetTableName(tt.model)
			if got != tt.want {
				t.Errorf("tryGetTableName() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_hasLifecycleHooks tests the hasLifecycleHooks function
func Test_hasLifecycleHooks(t *testing.T) {
	tests := []struct {
		name      string
		modelType reflect.Type
		want      bool
	}{
		{
			name:      "SimpleModel_NoHooks",
			modelType: reflect.TypeOf(SimpleModelForTest{}),
			want:      false,
		},
		{
			name:      "ModelWithHooks",
			modelType: reflect.TypeOf(ModelWithHooksForTest{}),
			want:      true,
		},
		{
			name:      "RelatedModel_WithHooks",
			modelType: reflect.TypeOf(RelatedModelForTest{}),
			want:      true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasLifecycleHooks(tt.modelType)
			if got != tt.want {
				t.Errorf("hasLifecycleHooks() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// Test_modelHasHooksAndRelationships tests the internal helper that detects
// models with both hooks and relationships
func Test_modelHasHooksAndRelationships(t *testing.T) {
	// Create a test context
	tc := NewDBTestContext(t)
	defer tc.Teardown()
	
	// Create a transaction helper to access the function
	th := NewTransactionTestHelper(tc)
	
	tests := []struct {
		name      string
		modelType reflect.Type
		want      bool
	}{
		{
			name:      "SimpleModel_NoRelationships",
			modelType: reflect.TypeOf(SimpleModelForTest{}),
			want:      false,
		},
		{
			name:      "ModelWithHooks_NoRelationships",
			modelType: reflect.TypeOf(ModelWithHooksForTest{}),
			want:      false,
		},
		{
			name:      "RelatedModel_WithHooksAndRelationships",
			modelType: reflect.TypeOf(RelatedModelForTest{}),
			want:      true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := th.modelHasHooksAndRelationships(tt.modelType)
			if got != tt.want {
				t.Errorf("modelHasHooksAndRelationships() got = %v, want %v", got, tt.want)
			}
		})
	}
}