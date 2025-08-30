package context

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultBuildSystemDetector implements build system detection for common project types
type DefaultBuildSystemDetector struct {
	detectors map[string]BuildSystemAnalyzer
}

// BuildSystemAnalyzer defines how to analyze a specific build system
type BuildSystemAnalyzer struct {
	Type       string
	ConfigFile string
	Parser     func(string) (*BuildSystem, error)
}

// NewBuildSystemDetector creates a new build system detector
func NewBuildSystemDetector() *DefaultBuildSystemDetector {
	detector := &DefaultBuildSystemDetector{
		detectors: make(map[string]BuildSystemAnalyzer),
	}
	
	detector.initializeBuildSystems()
	return detector
}

// DetectBuildSystems finds and analyzes build system configurations
func (d *DefaultBuildSystemDetector) DetectBuildSystems(workspacePath string) ([]BuildSystem, error) {
	var buildSystems []BuildSystem
	
	// Walk through the workspace looking for build system files
	err := filepath.WalkDir(workspacePath, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil // Continue on errors
		}
		
		if entry.IsDir() {
			return nil
		}
		
		// Check if this file matches any known build system
		filename := entry.Name()
		for _, analyzer := range d.detectors {
			if d.matchesPattern(filename, analyzer.ConfigFile) {
				if buildSystem, err := analyzer.Parser(path); err == nil {
					buildSystems = append(buildSystems, *buildSystem)
				}
			}
		}
		
		return nil
	})
	
	return buildSystems, err
}

// AnalyzeBuildFile parses a specific build configuration file
func (d *DefaultBuildSystemDetector) AnalyzeBuildFile(filePath string) (*BuildSystem, error) {
	filename := filepath.Base(filePath)
	
	for _, analyzer := range d.detectors {
		if d.matchesPattern(filename, analyzer.ConfigFile) {
			return analyzer.Parser(filePath)
		}
	}
	
	return nil, fmt.Errorf("unsupported build file: %s", filename)
}

// GetSupportedSystems returns a list of supported build systems
func (d *DefaultBuildSystemDetector) GetSupportedSystems() []string {
	var systems []string
	for _, analyzer := range d.detectors {
		systems = append(systems, analyzer.Type)
	}
	return systems
}

// initializeBuildSystems sets up analyzers for common build systems
func (d *DefaultBuildSystemDetector) initializeBuildSystems() {
	// Go modules
	d.detectors["go.mod"] = BuildSystemAnalyzer{
		Type:       "go",
		ConfigFile: "go.mod",
		Parser:     d.parseGoMod,
	}
	
	// Node.js / npm
	d.detectors["package.json"] = BuildSystemAnalyzer{
		Type:       "npm",
		ConfigFile: "package.json",
		Parser:     d.parsePackageJSON,
	}
	
	// Rust / Cargo
	d.detectors["Cargo.toml"] = BuildSystemAnalyzer{
		Type:       "cargo",
		ConfigFile: "Cargo.toml",
		Parser:     d.parseCargoToml,
	}
	
	// Python / pip
	d.detectors["requirements.txt"] = BuildSystemAnalyzer{
		Type:       "pip",
		ConfigFile: "requirements.txt",
		Parser:     d.parseRequirementsTxt,
	}
	
	// Python / Poetry
	d.detectors["pyproject.toml"] = BuildSystemAnalyzer{
		Type:       "poetry",
		ConfigFile: "pyproject.toml",
		Parser:     d.parsePyprojectToml,
	}
	
	// Python / setuptools
	d.detectors["setup.py"] = BuildSystemAnalyzer{
		Type:       "setuptools",
		ConfigFile: "setup.py",
		Parser:     d.parseSetupPy,
	}
	
	// Java / Maven
	d.detectors["pom.xml"] = BuildSystemAnalyzer{
		Type:       "maven",
		ConfigFile: "pom.xml",
		Parser:     d.parsePomXML,
	}
	
	// Java / Gradle
	d.detectors["build.gradle"] = BuildSystemAnalyzer{
		Type:       "gradle",
		ConfigFile: "build.gradle",
		Parser:     d.parseBuildGradle,
	}
	
	// C/C++ / CMake
	d.detectors["CMakeLists.txt"] = BuildSystemAnalyzer{
		Type:       "cmake",
		ConfigFile: "CMakeLists.txt",
		Parser:     d.parseCMakeLists,
	}
	
	// Generic Makefile
	d.detectors["Makefile"] = BuildSystemAnalyzer{
		Type:       "make",
		ConfigFile: "Makefile",
		Parser:     d.parseMakefile,
	}
	
	// PHP / Composer
	d.detectors["composer.json"] = BuildSystemAnalyzer{
		Type:       "composer",
		ConfigFile: "composer.json",
		Parser:     d.parseComposerJSON,
	}
	
	// Ruby / Bundler
	d.detectors["Gemfile"] = BuildSystemAnalyzer{
		Type:       "bundler",
		ConfigFile: "Gemfile",
		Parser:     d.parseGemfile,
	}
}

// Go module parser
func (d *DefaultBuildSystemDetector) parseGoMod(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	lines := strings.Split(string(content), "\n")
	buildSystem := &BuildSystem{
		Type:         "go",
		ConfigFile:   filePath,
		Dependencies: []string{},
		Scripts:      make(map[string]string),
	}
	
	inRequireBlock := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "module ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				modulePath := parts[1]
				buildSystem.Name = filepath.Base(modulePath)
			}
		}
		
		if strings.HasPrefix(line, "go ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				buildSystem.Version = parts[1]
			}
		}
		
		// Handle require block start
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		
		// Handle require block end
		if inRequireBlock && strings.Contains(line, ")") {
			inRequireBlock = false
			continue
		}
		
		// Parse single line require
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				buildSystem.Dependencies = append(buildSystem.Dependencies, parts[1])
			}
		}
		
		// Parse require block content
		if inRequireBlock && line != "" && !strings.HasPrefix(line, "//") {
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				buildSystem.Dependencies = append(buildSystem.Dependencies, parts[0])
			}
		}
	}
	
	// Add common Go commands
	buildSystem.Scripts["build"] = "go build"
	buildSystem.Scripts["test"] = "go test ./..."
	buildSystem.Scripts["run"] = "go run"
	
	return buildSystem, nil
}

// package.json parser
func (d *DefaultBuildSystemDetector) parsePackageJSON(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var pkg struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Scripts      map[string]string `json:"scripts"`
		Dependencies map[string]string `json:"dependencies"`
		DevDeps      map[string]string `json:"devDependencies"`
	}
	
	if err := json.Unmarshal(content, &pkg); err != nil {
		return nil, err
	}
	
	buildSystem := &BuildSystem{
		Type:         "npm",
		ConfigFile:   filePath,
		Name:         pkg.Name,
		Version:      pkg.Version,
		Scripts:      pkg.Scripts,
		Dependencies: []string{},
	}
	
	// Collect dependencies
	for dep := range pkg.Dependencies {
		buildSystem.Dependencies = append(buildSystem.Dependencies, dep)
	}
	for dep := range pkg.DevDeps {
		buildSystem.Dependencies = append(buildSystem.Dependencies, dep+" (dev)")
	}
	
	return buildSystem, nil
}

// Cargo.toml parser
func (d *DefaultBuildSystemDetector) parseCargoToml(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	// Simple parsing without external TOML library
	
	// Simple TOML parsing (basic implementation)
	lines := strings.Split(string(content), "\n")
	buildSystem := &BuildSystem{
		Type:         "cargo",
		ConfigFile:   filePath,
		Dependencies: []string{},
		Scripts:      make(map[string]string),
	}
	
	inPackage := false
	inDependencies := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "[package]" {
			inPackage = true
			inDependencies = false
			continue
		}
		
		if line == "[dependencies]" {
			inPackage = false
			inDependencies = true
			continue
		}
		
		if strings.HasPrefix(line, "[") {
			inPackage = false
			inDependencies = false
			continue
		}
		
		if inPackage {
			if strings.HasPrefix(line, "name = ") {
				buildSystem.Name = strings.Trim(strings.TrimPrefix(line, "name = "), "\"")
			}
			if strings.HasPrefix(line, "version = ") {
				buildSystem.Version = strings.Trim(strings.TrimPrefix(line, "version = "), "\"")
			}
		}
		
		if inDependencies && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				dep := strings.TrimSpace(parts[0])
				buildSystem.Dependencies = append(buildSystem.Dependencies, dep)
			}
		}
	}
	
	// Add common Cargo commands
	buildSystem.Scripts["build"] = "cargo build"
	buildSystem.Scripts["test"] = "cargo test"
	buildSystem.Scripts["run"] = "cargo run"
	
	return buildSystem, nil
}

// requirements.txt parser
func (d *DefaultBuildSystemDetector) parseRequirementsTxt(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	buildSystem := &BuildSystem{
		Type:         "pip",
		ConfigFile:   filePath,
		Name:         filepath.Base(filepath.Dir(filePath)),
		Dependencies: []string{},
		Scripts:      make(map[string]string),
	}
	
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Extract package name (before version specifier)
			parts := strings.FieldsFunc(line, func(c rune) bool {
				return c == '=' || c == '>' || c == '<' || c == '!' || c == '~'
			})
			if len(parts) > 0 {
				buildSystem.Dependencies = append(buildSystem.Dependencies, parts[0])
			}
		}
	}
	
	// Add common Python commands
	buildSystem.Scripts["install"] = "pip install -r requirements.txt"
	buildSystem.Scripts["test"] = "python -m pytest"
	
	return buildSystem, nil
}

// pyproject.toml parser (simplified)
func (d *DefaultBuildSystemDetector) parsePyprojectToml(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	buildSystem := &BuildSystem{
		Type:         "poetry",
		ConfigFile:   filePath,
		Dependencies: []string{},
		Scripts:      make(map[string]string),
	}
	
	// Simple parsing for [tool.poetry] section
	lines := strings.Split(string(content), "\n")
	inPoetry := false
	inDependencies := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if line == "[tool.poetry]" {
			inPoetry = true
			continue
		}
		
		if line == "[tool.poetry.dependencies]" {
			inDependencies = true
			continue
		}
		
		if strings.HasPrefix(line, "[") {
			inPoetry = false
			inDependencies = false
			continue
		}
		
		if inPoetry {
			if strings.HasPrefix(line, "name = ") {
				buildSystem.Name = strings.Trim(strings.TrimPrefix(line, "name = "), "\"")
			}
			if strings.HasPrefix(line, "version = ") {
				buildSystem.Version = strings.Trim(strings.TrimPrefix(line, "version = "), "\"")
			}
		}
		
		if inDependencies && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				dep := strings.TrimSpace(parts[0])
				if dep != "python" {
					buildSystem.Dependencies = append(buildSystem.Dependencies, dep)
				}
			}
		}
	}
	
	// Add common Poetry commands
	buildSystem.Scripts["install"] = "poetry install"
	buildSystem.Scripts["build"] = "poetry build"
	buildSystem.Scripts["test"] = "poetry run pytest"
	
	return buildSystem, nil
}

// Simplified parsers for other build systems
func (d *DefaultBuildSystemDetector) parseSetupPy(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "setuptools",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"build":   "python setup.py build",
			"install": "python setup.py install",
		},
	}, nil
}

func (d *DefaultBuildSystemDetector) parsePomXML(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "maven",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"build":   "mvn compile",
			"test":    "mvn test",
			"package": "mvn package",
		},
	}, nil
}

func (d *DefaultBuildSystemDetector) parseBuildGradle(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "gradle",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"build": "gradle build",
			"test":  "gradle test",
		},
	}, nil
}

func (d *DefaultBuildSystemDetector) parseCMakeLists(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "cmake",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"configure": "cmake .",
			"build":     "make",
		},
	}, nil
}

func (d *DefaultBuildSystemDetector) parseMakefile(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "make",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"build": "make",
			"clean": "make clean",
		},
	}, nil
}

func (d *DefaultBuildSystemDetector) parseComposerJSON(filePath string) (*BuildSystem, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var composer struct {
		Name         string            `json:"name"`
		Version      string            `json:"version"`
		Scripts      map[string]string `json:"scripts"`
		Require      map[string]string `json:"require"`
		RequireDev   map[string]string `json:"require-dev"`
	}
	
	if err := json.Unmarshal(content, &composer); err != nil {
		return nil, err
	}
	
	buildSystem := &BuildSystem{
		Type:         "composer",
		ConfigFile:   filePath,
		Name:         composer.Name,
		Version:      composer.Version,
		Scripts:      composer.Scripts,
		Dependencies: []string{},
	}
	
	for dep := range composer.Require {
		buildSystem.Dependencies = append(buildSystem.Dependencies, dep)
	}
	
	return buildSystem, nil
}

func (d *DefaultBuildSystemDetector) parseGemfile(filePath string) (*BuildSystem, error) {
	return &BuildSystem{
		Type:       "bundler",
		ConfigFile: filePath,
		Name:       filepath.Base(filepath.Dir(filePath)),
		Scripts: map[string]string{
			"install": "bundle install",
			"exec":    "bundle exec",
		},
	}, nil
}

// matchesPattern checks if a filename matches a pattern (supports simple wildcards)
func (d *DefaultBuildSystemDetector) matchesPattern(filename, pattern string) bool {
	if pattern == filename {
		return true
	}
	
	// Handle case-insensitive matching for common files
	if strings.EqualFold(filename, pattern) {
		return true
	}
	
	return false
}