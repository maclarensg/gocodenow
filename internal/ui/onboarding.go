package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// OnboardingStep represents a single step in the onboarding process
type OnboardingStep struct {
	ID          string
	Title       string
	Content     string
	Explanation string
	Action      string      // What the user should do
	ExpectedKey string      // Key they should press
	Completed   bool
	Duration    time.Duration // How long this step should be visible
}

// OnboardingFlow manages the complete onboarding experience
type OnboardingFlow struct {
	steps           []OnboardingStep
	currentStep     int
	active          bool
	completed       bool
	startTime       time.Time
	skipRequested   bool
	userProgress    map[string]bool // Track which steps user has completed
	autoAdvance     bool            // Whether to auto-advance through steps
	interactiveMode bool            // Whether user must complete actions to advance
}

// NewOnboardingFlow creates a new onboarding flow
func NewOnboardingFlow() *OnboardingFlow {
	flow := &OnboardingFlow{
		steps:           make([]OnboardingStep, 0),
		currentStep:     0,
		active:          false,
		completed:       false,
		userProgress:    make(map[string]bool),
		autoAdvance:     false,
		interactiveMode: true,
	}
	
	flow.initializeSteps()
	return flow
}

// initializeSteps sets up the complete onboarding sequence
func (of *OnboardingFlow) initializeSteps() {
	steps := []OnboardingStep{
		{
			ID:          "welcome",
			Title:       "Welcome to lmcodenow! 🚀",
			Content:     "lmcodenow is your AI-powered coding assistant that runs in your terminal. It can help you with coding problems, file operations, debugging, and much more.",
			Explanation: "This tutorial will guide you through the basic features in just a few steps.",
			Action:      "Press any key to continue",
			ExpectedKey: "any",
			Duration:    0, // Wait for user input
		},
		{
			ID:          "interface_overview",
			Title:       "Interface Overview",
			Content:     "The screen is divided into three main areas:\n• Header: Shows connection status and model info\n• Conversation Area: Displays your chat history\n• Input Area: Where you type your messages",
			Explanation: "You can see the input area at the bottom with helpful keyboard shortcuts listed.",
			Action:      "Press Tab to focus the input area",
			ExpectedKey: "tab",
			Duration:    0,
		},
		{
			ID:          "first_message",
			Title:       "Your First Message",
			Content:     "Now you're ready to send your first message to the AI! Type anything you'd like help with.",
			Explanation: "Try asking about coding problems, file operations, or general programming questions.\n\nExamples:\n• \"Help me create a Python web server\"\n• \"Explain this error message\"\n• \"Read the contents of package.json\"",
			Action:      "Type a message and press Enter to send it",
			ExpectedKey: "enter",
			Duration:    0,
		},
		{
			ID:          "navigation",
			Title:       "Navigating Conversations",
			Content:     "Great! Once you have some conversation history, you can navigate through it efficiently.",
			Explanation: "• Use ↑↓ arrows to browse through conversations\n• Press Space or Enter to expand/collapse conversations\n• Tab switches between conversation area and input\n• Home/End jump to first/last conversation",
			Action:      "Press Tab to focus the conversation area, then try the arrow keys",
			ExpectedKey: "tab",
			Duration:    0,
		},
		{
			ID:          "advanced_features",
			Title:       "Advanced Features",
			Content:     "lmcodenow has many powerful features to enhance your workflow:",
			Explanation: "• Press ? or F1 for complete keyboard shortcuts\n• Del or 'd' deletes conversations\n• Ctrl+Del clears all conversations (with confirmation)\n• The AI can read, write, and modify files in your project\n• Tool execution progress is shown in real-time",
			Action:      "Press ? to see the help panel",
			ExpectedKey: "?",
			Duration:    0,
		},
		{
			ID:          "completion",
			Title:       "Tutorial Complete! ✨",
			Content:     "You're now ready to use lmcodenow effectively. The AI assistant can help you with a wide range of tasks:",
			Explanation: "• Code generation and debugging\n• File operations and project management\n• Explaining complex concepts\n• Reviewing and optimizing code\n\nRemember: Press ? anytime for help, and tooltips will guide you along the way.",
			Action:      "Press any key to finish the tutorial",
			ExpectedKey: "any",
			Duration:    5 * time.Second, // Auto-advance after 5 seconds
		},
	}
	
	of.steps = steps
}

// Start begins the onboarding flow
func (of *OnboardingFlow) Start() {
	if of.completed {
		return // Don't restart if already completed
	}
	
	of.active = true
	of.currentStep = 0
	of.startTime = time.Now()
	of.skipRequested = false
}

// Skip allows the user to skip the onboarding
func (of *OnboardingFlow) Skip() {
	of.skipRequested = true
	of.active = false
	of.completed = true
}

// IsActive returns whether the onboarding is currently active
func (of *OnboardingFlow) IsActive() bool {
	return of.active && !of.completed && !of.skipRequested
}

// IsCompleted returns whether the onboarding has been completed
func (of *OnboardingFlow) IsCompleted() bool {
	return of.completed
}

// GetCurrentStep returns the current onboarding step
func (of *OnboardingFlow) GetCurrentStep() *OnboardingStep {
	if !of.IsActive() || of.currentStep >= len(of.steps) {
		return nil
	}
	return &of.steps[of.currentStep]
}

// HandleKeyMsg processes key input for the onboarding flow
func (of *OnboardingFlow) HandleKeyMsg(msg tea.KeyMsg) tea.Cmd {
	if !of.IsActive() {
		return nil
	}
	
	keyStr := msg.String()
	
	// Allow skipping with Esc
	if keyStr == "esc" {
		of.Skip()
		return nil
	}
	
	currentStep := of.GetCurrentStep()
	if currentStep == nil {
		return nil
	}
	
	// Check if the user pressed the expected key
	expectedKey := currentStep.ExpectedKey
	canAdvance := false
	
	switch expectedKey {
	case "any":
		canAdvance = true
	case keyStr:
		canAdvance = true
		of.userProgress[currentStep.ID] = true
	default:
		// For interactive mode, only advance if they pressed the right key
		if !of.interactiveMode {
			canAdvance = true
		}
	}
	
	if canAdvance {
		return of.AdvanceStep()
	}
	
	return nil
}

// AdvanceStep moves to the next step in the onboarding
func (of *OnboardingFlow) AdvanceStep() tea.Cmd {
	if !of.IsActive() {
		return nil
	}
	
	// Mark current step as completed
	if of.currentStep < len(of.steps) {
		of.steps[of.currentStep].Completed = true
	}
	
	of.currentStep++
	
	// Check if we've completed all steps
	if of.currentStep >= len(of.steps) {
		of.completed = true
		of.active = false
		return func() tea.Msg {
			return OnboardingCompletedMsg{
				Duration:    time.Since(of.startTime),
				StepsShown:  len(of.steps),
				UserSkipped: of.skipRequested,
			}
		}
	}
	
	// If the new step has auto-advance, set a timer
	newStep := &of.steps[of.currentStep]
	if newStep.Duration > 0 {
		return tea.Tick(newStep.Duration, func(time.Time) tea.Msg {
			return OnboardingStepTimeoutMsg{StepID: newStep.ID}
		})
	}
	
	return nil
}

// HandleTimeout handles step timeouts for auto-advancing steps
func (of *OnboardingFlow) HandleTimeout(msg OnboardingStepTimeoutMsg) tea.Cmd {
	if !of.IsActive() {
		return nil
	}
	
	currentStep := of.GetCurrentStep()
	if currentStep != nil && currentStep.ID == msg.StepID {
		return of.AdvanceStep()
	}
	
	return nil
}

// Render displays the current onboarding step
func (of *OnboardingFlow) Render(width, height int) string {
	if !of.IsActive() {
		return ""
	}
	
	currentStep := of.GetCurrentStep()
	if currentStep == nil {
		return ""
	}
	
	return of.renderStep(*currentStep, width, height)
}

// renderStep renders a single onboarding step
func (of *OnboardingFlow) renderStep(step OnboardingStep, width, height int) string {
	// Calculate overlay size (80% of screen)
	overlayWidth := int(float64(width) * 0.8)
	overlayHeight := int(float64(height) * 0.6)
	
	// Ensure minimum size
	if overlayWidth < 50 {
		overlayWidth = min(width-4, 50)
	}
	if overlayHeight < 10 {
		overlayHeight = min(height-4, 10)
	}
	
	// Define styles
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00D7FF")).
		Background(lipgloss.Color("#001122")).
		Padding(2).
		Width(overlayWidth).
		Height(overlayHeight).
		Align(lipgloss.Left)
	
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00D7FF")).
		Bold(true).
		Align(lipgloss.Center).
		Width(overlayWidth - 6)
	
	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Width(overlayWidth - 6)
	
	explanationStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC")).
		Italic(true).
		Width(overlayWidth - 6)
	
	actionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		Bold(true).
		Width(overlayWidth - 6)
	
	progressStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Width(overlayWidth - 6)
	
	// Build step content
	var content strings.Builder
	
	// Title
	content.WriteString(titleStyle.Render(step.Title))
	content.WriteString("\n\n")
	
	// Content
	content.WriteString(contentStyle.Render(step.Content))
	content.WriteString("\n\n")
	
	// Explanation
	if step.Explanation != "" {
		content.WriteString(explanationStyle.Render(step.Explanation))
		content.WriteString("\n\n")
	}
	
	// Action prompt
	content.WriteString(actionStyle.Render("→ " + step.Action))
	content.WriteString("\n\n")
	
	// Progress indicator
	progress := of.getProgressIndicator()
	content.WriteString(progressStyle.Render(progress))
	
	// Skip instruction
	content.WriteString("\n")
	skipStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)
	content.WriteString(skipStyle.Render("Press Esc to skip tutorial"))
	
	// Render the bordered overlay
	overlay := borderStyle.Render(content.String())
	
	// Center the overlay
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)
}

// getProgressIndicator creates a progress indicator for the current step
func (of *OnboardingFlow) getProgressIndicator() string {
	if len(of.steps) == 0 {
		return ""
	}
	
	current := of.currentStep + 1
	total := len(of.steps)
	
	// Create a simple text progress indicator
	percentage := int(float64(current) / float64(total) * 100)
	
	// Create progress bar
	barWidth := 20
	filledWidth := int(float64(barWidth) * float64(current) / float64(total))
	
	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		if i < filledWidth {
			bar.WriteString("█")
		} else {
			bar.WriteString("░")
		}
	}
	bar.WriteString("]")
	
	return lipgloss.JoinHorizontal(
		lipgloss.Left,
		bar.String(),
		" ",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")).Render(
			lipgloss.JoinHorizontal(lipgloss.Left,
				" Step ", 
				lipgloss.NewStyle().Bold(true).Render(lipgloss.JoinHorizontal(lipgloss.Left, 
					lipgloss.JoinHorizontal(lipgloss.Left, string(rune('0'+current/10)), string(rune('0'+current%10))), 
					"/", 
					lipgloss.JoinHorizontal(lipgloss.Left, string(rune('0'+total/10)), string(rune('0'+total%10))))),
				" (", 
				lipgloss.JoinHorizontal(lipgloss.Left, string(rune('0'+percentage/100)), string(rune('0'+(percentage%100)/10)), string(rune('0'+percentage%10))), 
				"%)"),
		),
	)
}

// GetStepCount returns the total number of onboarding steps
func (of *OnboardingFlow) GetStepCount() int {
	return len(of.steps)
}

// GetCompletedStepCount returns the number of completed steps
func (of *OnboardingFlow) GetCompletedStepCount() int {
	count := 0
	for _, step := range of.steps {
		if step.Completed {
			count++
		}
	}
	return count
}

// ShouldShowForFirstTime determines if onboarding should be shown for new users
func (of *OnboardingFlow) ShouldShowForFirstTime(conversationCount int) bool {
	// Show onboarding if:
	// 1. Not already completed
	// 2. User hasn't skipped it
	// 3. There are no existing conversations (first time user)
	return !of.completed && !of.skipRequested && conversationCount == 0
}

// OnboardingCompletedMsg is sent when onboarding is completed
type OnboardingCompletedMsg struct {
	Duration    time.Duration
	StepsShown  int
	UserSkipped bool
}

// OnboardingStepTimeoutMsg is sent when a step times out
type OnboardingStepTimeoutMsg struct {
	StepID string
}

// OnboardingKeyActionMsg represents a key action during onboarding
type OnboardingKeyActionMsg struct {
	StepID string
	Key    string
	Valid  bool
}