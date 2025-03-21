// Package tabletest provides models for testing table resolution fixes.
// This package specifically creates models with cross-package relationships and hooks
// that would trigger the "Table not set" bug in GORM.
package tabletest

import (
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

// ExternalModel is a model from an "external" package that has hooks using map operations
type ExternalModel struct {
	ID              uint `gorm:"primarykey"`
	ReferenceNumber string
	Description     string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// TableName returns the table name
func (ExternalModel) TableName() string {
	return "external_models"
}

// BeforeSave hook intentionally uses map operations to trigger the bug
func (e *ExternalModel) BeforeSave(tx *gorm.DB) error {
	e.ReferenceNumber = "REF-" + e.ReferenceNumber

	// This map operation is important for triggering the bug
	validMap := map[string]bool{
		"REF-TEST-123": true,
		"REF-DEMO":     true,
	}

	// Simply accessing the map helps trigger the bug
	_ = validMap[e.ReferenceNumber]

	return nil
}

// RelatedModel represents a model with a relationship to the external model
type RelatedModel struct {
	ID        uint `gorm:"primarykey"`
	ModelID   uint
	Model     ExternalModel `gorm:"foreignKey:ModelID"` // Cross-package relationship
	Message   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name
func (RelatedModel) TableName() string {
	return "related_models"
}

// BeforeCreate uses map operations to trigger the bug
func (r *RelatedModel) BeforeCreate(tx *gorm.DB) error {
	// This map operation in a hook on a related model
	// significantly increases chances of triggering the bug
	msgMap := map[string]bool{
		"test": true,
		"demo": true,
	}

	if len(r.Message) > 0 && msgMap[r.Message] {
		r.Message = "Validated: " + r.Message
	}

	return nil
}

// ComplexModel has cross-package relationships and validates related content
type ComplexModel struct {
	gorm.Model
	ExternalID uint
	External   ExternalModel `gorm:"foreignKey:ExternalID"` // Cross-package relationship
	RelatedID  uint
	Related    RelatedModel `gorm:"foreignKey:RelatedID"` // Another cross-package relationship
	Name       string
}

// TableName returns the table name
func (ComplexModel) TableName() string {
	return "complex_models"
}

// BeforeCreate validates the model before creation
func (c *ComplexModel) BeforeCreate(tx *gorm.DB) error {
	return c.Validate()
}

// BeforeUpdate validates the model before update
func (c *ComplexModel) BeforeUpdate(tx *gorm.DB) error {
	return c.Validate()
}

// Validate validates a Complex model with map operations
func (c *ComplexModel) Validate() error {
	// Validate Name
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}

	// Use a map operation to trigger the bug
	validModels := map[uint]bool{
		c.ExternalID: true,
		c.RelatedID:  true,
	}

	// Check if models are valid
	if !validModels[c.ExternalID] || !validModels[c.RelatedID] {
		return fmt.Errorf("invalid model references")
	}

	return nil
}

// SimpleModel does not have relationships or complex hooks
type SimpleModel struct {
	gorm.Model
	Name string
}

// TableName returns the table name
func (SimpleModel) TableName() string {
	return "simple_models"
}
