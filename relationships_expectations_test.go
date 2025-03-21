package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// RelTestParent and RelTestChild models for relationship testing
type RelTestParent struct {
	gorm.Model
	Name     string
	Children []RelTestChild `gorm:"foreignKey:ParentID"`
}

type RelTestChild struct {
	gorm.Model
	Name     string
	ParentID uint
	Parent   RelTestParent `gorm:"foreignKey:ParentID"`
}

func (RelTestParent) TableName() string {
	return "rel_test_parents"
}

func (RelTestChild) TableName() string {
	return "rel_test_children"
}

// TestFindExpectationWithRelationships tests the high-level API with relationship models
func TestFindExpectationWithRelationships(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	RegisterModelWithRelationships[RelTestParent](tc)

	now := time.Now()
	parent := RelTestParent{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent 1",
		Children: []RelTestChild{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child 1",
				ParentID: 1,
			},
		},
	}

	tc.ForTable("rel_test_parents").
		ExpectFind().
		ReturnModels([]RelTestParent{parent})

	var results []RelTestParent
	err := tc.DB().Table("rel_test_parents").Find(&results).Error

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Parent 1", results[0].Name)
}

// TestSearchWithRelationships tests using direct Where clause with relationship models
func TestSearchWithRelationships(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	RegisterModelWithRelationships[RelTestParent](tc)

	now := time.Now()
	parent := RelTestParent{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent Search",
		Children: []RelTestChild{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child Search",
				ParentID: 1,
			},
		},
	}

	// Using Find with Where is more predictable than ExpectSearch
	tc.ForTable("rel_test_parents").
		ExpectFind().
		Where("name LIKE ?").
		WithArgs("%Search%").
		ReturnModels([]RelTestParent{parent})

	var results []RelTestParent
	tc.DB().Table("rel_test_parents").Where("name LIKE ?", "%Search%").Find(&results)

	assert.Len(t, results, 1)
	assert.Equal(t, "Parent Search", results[0].Name)
}

// TestQueryExpectationWithRelationships tests the QueryExpectationBuilder with relationship models
func TestQueryExpectationWithRelationships(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	RegisterModelWithRelationships[RelTestParent](tc)

	now := time.Now()
	parent := RelTestParent{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent Query",
		Children: []RelTestChild{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child Query",
				ParentID: 1,
			},
		},
	}

	tc.Expect().
		Query("SELECT \\* FROM `rel_test_parents` WHERE id = \\?").
		WithArgs(1).
		ReturnModels("rel_test_parents", []RelTestParent{parent})

	var results []RelTestParent
	tc.DB().Raw("SELECT * FROM `rel_test_parents` WHERE id = ?", 1).Scan(&results)

	assert.Len(t, results, 1)
	assert.Equal(t, "Parent Query", results[0].Name)
}

// TestFilterWithRelationships tests WHERE filters with relationship models
func TestFilterWithRelationships(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	RegisterModelWithRelationships[RelTestParent](tc)

	now := time.Now()
	parent := RelTestParent{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent Filter",
		Children: []RelTestChild{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child Filter",
				ParentID: 1,
			},
		},
	}

	tc.ForTable("rel_test_parents").
		ExpectFind().
		Where("name = ?").
		WithArgs("Parent Filter").
		ReturnModels([]RelTestParent{parent})

	var results []RelTestParent
	tc.DB().Table("rel_test_parents").Where("name = ?", "Parent Filter").Find(&results)

	assert.Len(t, results, 1)
	assert.Equal(t, "Parent Filter", results[0].Name)
}
