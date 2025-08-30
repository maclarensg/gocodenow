package tools

import (
	"context"
	"fmt"
)

// BaseExecutor provides common functionality for tool executors
type BaseExecutor struct {
	name        string
	description string
	schema      ToolSchema
}

// NewBaseExecutor creates a new base executor
func NewBaseExecutor(name, description string) *BaseExecutor {
	return &BaseExecutor{
		name:        name,
		description: description,
		schema: ToolSchema{
			Name:        name,
			Description: description,
			Parameters:  make(map[string]interface{}),
			Required:    []string{},
			Examples:    []ToolParameters{},
		},
	}
}

// Name returns the name of this tool
func (b *BaseExecutor) Name() string {
	return b.name
}

// Description returns the description of this tool
func (b *BaseExecutor) Description() string {
	return b.description
}

// Schema returns the schema for this tool
func (b *BaseExecutor) Schema() ToolSchema {
	return b.schema
}

// SetSchema allows updating the schema
func (b *BaseExecutor) SetSchema(schema ToolSchema) {
	b.schema = schema
	b.schema.Name = b.name
	b.schema.Description = b.description
}

// Execute is a default implementation that should be overridden
func (b *BaseExecutor) Execute(ctx context.Context, params ToolParameters) (*ToolResult, error) {
	return nil, &ToolExecutionError{
		ToolName:    b.name,
		Message:     "execute method not implemented",
		Recoverable: false,
	}
}

// ValidateParameters provides basic parameter validation
func (b *BaseExecutor) ValidateParameters(params ToolParameters) error {
	if params == nil {
		return &ToolValidationError{
			ToolName: b.name,
			Message:  "parameters cannot be nil",
		}
	}

	// Check required parameters
	for _, required := range b.schema.Required {
		if _, exists := params[required]; !exists {
			return &ToolValidationError{
				ToolName:  b.name,
				Parameter: required,
				Message:   fmt.Sprintf("required parameter '%s' is missing", required),
			}
		}
	}

	return nil
}

// AddRequiredParam adds a required parameter to the schema
func (b *BaseExecutor) AddRequiredParam(name, paramType, description string) {
	if b.schema.Parameters == nil {
		b.schema.Parameters = make(map[string]interface{})
	}
	
	b.schema.Parameters[name] = map[string]interface{}{
		"type":        paramType,
		"description": description,
	}
	
	// Add to required list if not already present
	for _, req := range b.schema.Required {
		if req == name {
			return
		}
	}
	b.schema.Required = append(b.schema.Required, name)
}

// AddOptionalParam adds an optional parameter to the schema
func (b *BaseExecutor) AddOptionalParam(name, paramType, description string, defaultValue interface{}) {
	if b.schema.Parameters == nil {
		b.schema.Parameters = make(map[string]interface{})
	}
	
	paramSchema := map[string]interface{}{
		"type":        paramType,
		"description": description,
	}
	
	if defaultValue != nil {
		paramSchema["default"] = defaultValue
	}
	
	b.schema.Parameters[name] = paramSchema
}

// AddExample adds an example to the schema
func (b *BaseExecutor) AddExample(params ToolParameters) {
	b.schema.Examples = append(b.schema.Examples, params)
}