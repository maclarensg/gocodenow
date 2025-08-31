package app

import (
	"context"
	"time"

	"gocodenow/internal/config"
	"gocodenow/internal/llm"
	"gocodenow/internal/security"
	"gocodenow/internal/tools"
	"gocodenow/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// App represents the main application
type App struct {
	program    *tea.Program
	config     *config.Config
	llmClient  llm.Client
	llmFactory *llm.ClientFactory
}

// ConnectionStatus represents the connection state
type ConnectionStatus struct {
	Connected bool
	ModelName string
	Error     string
}

// New creates a new application with the new config system
func New(cfg *config.Config) *App {
	// Create LLM client factory
	factory := llm.NewClientFactory()
	
	// Test connection to LLM endpoint using new client
	connStatus := testLLMConnection(&cfg.LLM, factory)
	
	// Convert to UI ConnectionStatus type
	uiConnStatus := ui.ConnectionStatus{
		Connected: connStatus.Connected,
		ModelName: connStatus.ModelName,
		Error:     connStatus.Error,
	}
	
	// Create UI model
	model := ui.New(cfg.LLM.Model, uiConnStatus)
	
	// Initialize MessageProcessor if LLM is available
	if connStatus.Connected {
		// Create LLM client
		llmClient, err := factory.CreateClient(&cfg.LLM)
		if err == nil {
			// Get the conversation history from the UI model (they must share the same instance!)
			conversationHistory := model.GetConversationHistory()
			
			// Create tool router - for now empty, but this is where file/git/bash tools would be registered
			toolRouter := tools.NewToolRouter()
			
			// Create security services - for now nil, but these would handle confirmations and resource monitoring
			var securityService *security.ConfirmationService = nil
			var resourceMonitor *security.ResourceMonitor = nil
			
			// Create message processor using the SAME conversation history as the UI
			messageProcessor := ui.NewMessageProcessor(
				llmClient,
				toolRouter,
				conversationHistory,
				securityService,
				resourceMonitor,
			)
			
			// Set the message processor on the UI model
			model.SetMessageProcessor(messageProcessor)
		}
	}
	
	program := tea.NewProgram(model)
	
	return &App{
		program:    program,
		config:     cfg,
		llmFactory: factory,
	}
}

// testLLMConnection tests if we can connect to the LLM endpoint using the new client
func testLLMConnection(config *config.LLMConfig, factory *llm.ClientFactory) ConnectionStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Try to create an LLM client
	client, err := factory.CreateClient(config)
	if err != nil {
		return ConnectionStatus{
			Connected: false,
			ModelName: config.Model,
			Error:     "Failed to create LLM client: " + err.Error(),
		}
	}
	defer client.Close()
	
	// Try to list models to test connection
	modelsResp, err := client.ListModels(ctx)
	if err != nil {
		return ConnectionStatus{
			Connected: false,
			ModelName: config.Model,
			Error:     "Unable to connect: " + err.Error(),
		}
	}
	
	// Check if the requested model exists
	if config.Model != "" {
		for _, model := range modelsResp.Data {
			if model.ID == config.Model {
				return ConnectionStatus{
					Connected: true,
					ModelName: config.Model,
					Error:     "",
				}
			}
		}
		
		// Model not found
		return ConnectionStatus{
			Connected: false,
			ModelName: config.Model,
			Error:     "Model not found",
		}
	}
	
	// No specific model requested, connection successful
	return ConnectionStatus{
		Connected: true,
		ModelName: config.Model,
		Error:     "",
	}
}

// GetLLMClient returns a configured LLM client
func (a *App) GetLLMClient() (llm.Client, error) {
	if a.llmClient == nil {
		// Create client if not already created
		client, err := a.llmFactory.CreateClient(&a.config.LLM)
		if err != nil {
			return nil, err
		}
		a.llmClient = client
	}
	return a.llmClient, nil
}

// CloseLLMClient closes the LLM client if it exists
func (a *App) CloseLLMClient() error {
	if a.llmClient != nil {
		err := a.llmClient.Close()
		a.llmClient = nil
		return err
	}
	return nil
}

// Run starts the application
func (a *App) Run() error {
	defer a.CloseLLMClient() // Ensure client is closed when app exits
	_, err := a.program.Run()
	return err
}