package tools

import (
	"context"
	"testing"
)

func TestBaseExecutor_Basic(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool for unit testing")

	// Test basic properties
	if base.Name() != "test_tool" {
		t.Errorf("Expected name 'test_tool', got '%s'", base.Name())
	}

	if base.Description() != "A test tool for unit testing" {
		t.Errorf("Expected description 'A test tool for unit testing', got '%s'", base.Description())
	}

	schema := base.Schema()
	if schema.Name != "test_tool" {
		t.Errorf("Expected schema name 'test_tool', got '%s'", schema.Name)
	}

	if schema.Description != "A test tool for unit testing" {
		t.Errorf("Expected schema description 'A test tool for unit testing', got '%s'", schema.Description)
	}
}

func TestBaseExecutor_Execute_NotImplemented(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")
	ctx := context.Background()

	params := ToolParameters{}

	result, err := base.Execute(ctx, params)
	if err == nil {
		t.Error("Expected error for unimplemented Execute method")
	}

	if result != nil {
		t.Error("Expected nil result for unimplemented Execute method")
	}

	// Should be a ToolExecutionError
	if execError, ok := err.(*ToolExecutionError); ok {
		if execError.Recoverable {
			t.Error("Expected non-recoverable error for unimplemented method")
		}
		if execError.ToolName != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", execError.ToolName)
		}
	} else {
		t.Errorf("Expected ToolExecutionError, got %T", err)
	}
}

func TestBaseExecutor_ValidateParameters_Basic(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Test nil parameters
	err := base.ValidateParameters(nil)
	if err == nil {
		t.Error("Expected error for nil parameters")
	}

	// Test empty parameters (should be valid if no required params)
	params := ToolParameters{}
	err = base.ValidateParameters(params)
	if err != nil {
		t.Errorf("Expected no error for empty parameters, got: %v", err)
	}
}

func TestBaseExecutor_AddRequiredParam(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Add a required parameter
	base.AddRequiredParam("test_param", "string", "A test parameter")

	schema := base.Schema()

	// Check that parameter is in schema
	if _, exists := schema.Parameters["test_param"]; !exists {
		t.Error("Expected test_param to be in schema parameters")
	}

	// Check that parameter is in required list
	found := false
	for _, required := range schema.Required {
		if required == "test_param" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected test_param to be in required list")
	}

	// Test validation with missing required parameter
	params := ToolParameters{}
	err := base.ValidateParameters(params)
	if err == nil {
		t.Error("Expected error for missing required parameter")
	}

	// Test validation with present required parameter
	params = ToolParameters{
		"test_param": "test_value",
	}
	err = base.ValidateParameters(params)
	if err != nil {
		t.Errorf("Expected no error with required parameter present, got: %v", err)
	}
}

func TestBaseExecutor_AddOptionalParam(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Add an optional parameter with default value
	base.AddOptionalParam("optional_param", "integer", "An optional parameter", 42)

	schema := base.Schema()

	// Check that parameter is in schema
	paramSchema, exists := schema.Parameters["optional_param"]
	if !exists {
		t.Error("Expected optional_param to be in schema parameters")
	}

	// Check parameter schema structure
	if paramMap, ok := paramSchema.(map[string]interface{}); ok {
		if paramMap["type"] != "integer" {
			t.Errorf("Expected type 'integer', got %v", paramMap["type"])
		}
		if paramMap["default"] != 42 {
			t.Errorf("Expected default value 42, got %v", paramMap["default"])
		}
		if paramMap["description"] != "An optional parameter" {
			t.Errorf("Expected description 'An optional parameter', got %v", paramMap["description"])
		}
	} else {
		t.Errorf("Expected parameter schema to be map[string]interface{}, got %T", paramSchema)
	}

	// Check that parameter is NOT in required list
	for _, required := range schema.Required {
		if required == "optional_param" {
			t.Error("Optional parameter should not be in required list")
		}
	}

	// Test validation without optional parameter (should pass)
	params := ToolParameters{}
	err := base.ValidateParameters(params)
	if err != nil {
		t.Errorf("Expected no error without optional parameter, got: %v", err)
	}
}

func TestBaseExecutor_AddRequiredParam_Duplicate(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Add the same required parameter twice
	base.AddRequiredParam("test_param", "string", "First description")
	base.AddRequiredParam("test_param", "integer", "Second description")

	schema := base.Schema()

	// Should have only one entry in required list
	count := 0
	for _, required := range schema.Required {
		if required == "test_param" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Expected test_param to appear once in required list, found %d times", count)
	}

	// Parameter should have the latest type/description
	paramSchema, exists := schema.Parameters["test_param"]
	if !exists {
		t.Error("Expected test_param to be in schema parameters")
	}

	if paramMap, ok := paramSchema.(map[string]interface{}); ok {
		if paramMap["type"] != "integer" {
			t.Errorf("Expected type 'integer' (latest), got %v", paramMap["type"])
		}
		if paramMap["description"] != "Second description" {
			t.Errorf("Expected description 'Second description' (latest), got %v", paramMap["description"])
		}
	}
}

func TestBaseExecutor_AddExample(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Add some examples
	example1 := ToolParameters{
		"param1": "value1",
		"param2": 42,
	}
	example2 := ToolParameters{
		"param1": "value2",
		"param3": true,
	}

	base.AddExample(example1)
	base.AddExample(example2)

	schema := base.Schema()

	if len(schema.Examples) != 2 {
		t.Errorf("Expected 2 examples, got %d", len(schema.Examples))
	}

	// Check first example
	if schema.Examples[0]["param1"] != "value1" {
		t.Errorf("Expected first example param1 'value1', got %v", schema.Examples[0]["param1"])
	}

	// Check second example
	if schema.Examples[1]["param3"] != true {
		t.Errorf("Expected second example param3 true, got %v", schema.Examples[1]["param3"])
	}
}

func TestBaseExecutor_SetSchema(t *testing.T) {
	base := NewBaseExecutor("test_tool", "A test tool")

	// Create a custom schema
	customSchema := ToolSchema{
		Name:        "custom_name", // This should be overridden
		Description: "custom_description", // This should be overridden
		Parameters: map[string]interface{}{
			"custom_param": map[string]interface{}{
				"type":        "string",
				"description": "A custom parameter",
			},
		},
		Required: []string{"custom_param"},
		Examples: []ToolParameters{
			{"custom_param": "example_value"},
		},
	}

	base.SetSchema(customSchema)

	schema := base.Schema()

	// Name and description should be preserved from the base executor
	if schema.Name != "test_tool" {
		t.Errorf("Expected schema name 'test_tool', got '%s'", schema.Name)
	}

	if schema.Description != "A test tool" {
		t.Errorf("Expected schema description 'A test tool', got '%s'", schema.Description)
	}

	// Other fields should come from the custom schema
	if _, exists := schema.Parameters["custom_param"]; !exists {
		t.Error("Expected custom_param to be in schema parameters")
	}

	if len(schema.Required) != 1 || schema.Required[0] != "custom_param" {
		t.Errorf("Expected required ['custom_param'], got %v", schema.Required)
	}

	if len(schema.Examples) != 1 {
		t.Errorf("Expected 1 example, got %d", len(schema.Examples))
	}
}