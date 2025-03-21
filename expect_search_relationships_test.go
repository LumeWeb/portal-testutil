package testutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// TestExpectSearchWithRelationships tests ExpectSearch with relationship models
func TestExpectSearchWithRelationships(t *testing.T) {
	tc := NewDBTestContext(t)
	defer tc.Teardown()

	RegisterModelWithRelationships[RelTestParent](tc)

	now := time.Now()
	parent := RelTestParent{
		Model: gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		Name:  "Parent ExpectSearch",
		Children: []RelTestChild{
			{
				Model:    gorm.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
				Name:     "Child ExpectSearch",
				ParentID: 1,
			},
		},
	}

	tc.ForTable("rel_test_parents").
		ExpectSearch("ExpectSearch").
		WithFields("name").
		ReturnModels([]RelTestParent{parent})

	var results []RelTestParent
	tc.DB().Table("rel_test_parents").Where("name LIKE ?", "%ExpectSearch%").Find(&results)

	assert.Len(t, results, 1)
	assert.Equal(t, "Parent ExpectSearch", results[0].Name)
}
