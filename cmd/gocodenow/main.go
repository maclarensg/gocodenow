package main

import (
	"flag"
	"fmt"
	"os"

	"gocodenow/internal/app"
	"gocodenow/internal/config"
)

func main() {
	// Load configuration from files first
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	// Define CLI flags with defaults from loaded config
	var (
		endpoint   = flag.String("endpoint", cfg.LLM.Endpoint, "LLM API endpoint URL")
		token      = flag.String("token", cfg.LLM.Token, "API token (optional)")
		model      = flag.String("model", cfg.LLM.Model, "Model name")
		configFile = flag.String("config", "", "Path to configuration file")
	)
	
	// Support short flags
	flag.StringVar(endpoint, "e", cfg.LLM.Endpoint, "LLM API endpoint URL (short)")
	flag.StringVar(token, "t", cfg.LLM.Token, "API token (short)")
	flag.StringVar(model, "m", cfg.LLM.Model, "Model name (short)")
	
	// Parse flags
	flag.Parse()

	// Load from specific config file if provided
	if *configFile != "" {
		if fileCfg, err := config.LoadConfigFromFile(*configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config file %s: %v\n", *configFile, err)
			os.Exit(1)
		} else {
			cfg = fileCfg
		}
	}
	
	// Apply CLI flag overrides
	config.ApplyCLIFlags(cfg, endpoint, token, model)

	// Validate final configuration
	if err := config.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(1)
	}
	
	// Create application with merged configuration
	application := app.New(cfg)
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}