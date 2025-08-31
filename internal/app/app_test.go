package app

import (
	"testing"

	"gocodenow/internal/config"
)

func TestNew(t *testing.T) {
	// Create test config
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Endpoint: "http://test.example.com",
			Token:    "test-key",
			Model:    "test-model",
		},
	}

	// Test app creation
	app := New(cfg)

	if app == nil {
		t.Error("App should not be nil")
	}

	if app.config != cfg {
		t.Error("App config should match provided config")
	}

	if app.program == nil {
		t.Error("App program should not be nil")
	}

	if app.llmFactory == nil {
		t.Error("App LLM factory should not be nil")
	}
}

func TestConnectionStatus(t *testing.T) {
	status := ConnectionStatus{
		Connected: true,
		ModelName: "test-model",
		Error:     "",
	}

	if !status.Connected {
		t.Error("Connection status should be connected")
	}

	if status.ModelName != "test-model" {
		t.Errorf("Model name = %v, want test-model", status.ModelName)
	}

	if status.Error != "" {
		t.Errorf("Error should be empty, got %v", status.Error)
	}
}

func TestConnectionStatusWithError(t *testing.T) {
	status := ConnectionStatus{
		Connected: false,
		ModelName: "",
		Error:     "connection failed",
	}

	if status.Connected {
		t.Error("Connection status should not be connected")
	}

	if status.Error != "connection failed" {
		t.Errorf("Error = %v, want 'connection failed'", status.Error)
	}
}