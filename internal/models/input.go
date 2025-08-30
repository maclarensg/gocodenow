package models

import "strings"

// InputState manages the multiline input state
type InputState struct {
	lines       []string
	cursorLine  int
	cursorPos   int
	scrollOffset int
}

// NewInputState creates a new input state
func NewInputState() *InputState {
	return &InputState{
		lines:       []string{""},
		cursorLine:  0,
		cursorPos:   0,
		scrollOffset: 0,
	}
}

// GetLines returns all input lines
func (is *InputState) GetLines() []string {
	return is.lines
}

// GetCursorLine returns the current cursor line
func (is *InputState) GetCursorLine() int {
	return is.cursorLine
}

// GetCursorPos returns the current cursor position
func (is *InputState) GetCursorPos() int {
	return is.cursorPos
}

// GetScrollOffset returns the current scroll offset
func (is *InputState) GetScrollOffset() int {
	return is.scrollOffset
}

// GetText returns the complete input as a single string
func (is *InputState) GetText() string {
	return strings.Join(is.lines, "\n")
}

// Clear resets the input state
func (is *InputState) Clear() {
	is.lines = []string{""}
	is.cursorLine = 0
	is.cursorPos = 0
	is.scrollOffset = 0
}

// MoveCursorUp moves the cursor up one line
func (is *InputState) MoveCursorUp() {
	if is.cursorLine > 0 {
		is.cursorLine--
		if is.cursorPos > len(is.lines[is.cursorLine]) {
			is.cursorPos = len(is.lines[is.cursorLine])
		}
		is.ensureCursorVisible()
	}
}

// MoveCursorDown moves the cursor down one line
func (is *InputState) MoveCursorDown() {
	if is.cursorLine < len(is.lines)-1 {
		is.cursorLine++
		if is.cursorPos > len(is.lines[is.cursorLine]) {
			is.cursorPos = len(is.lines[is.cursorLine])
		}
		is.ensureCursorVisible()
	}
}

// MoveCursorLeft moves the cursor left one position
func (is *InputState) MoveCursorLeft() {
	if is.cursorPos > 0 {
		is.cursorPos--
	}
}

// MoveCursorRight moves the cursor right one position
func (is *InputState) MoveCursorRight() {
	if is.cursorPos < len(is.lines[is.cursorLine]) {
		is.cursorPos++
	}
}

// InsertChar inserts a character at the cursor position
func (is *InputState) InsertChar(char string) {
	line := is.lines[is.cursorLine]
	is.lines[is.cursorLine] = line[:is.cursorPos] + char + line[is.cursorPos:]
	is.cursorPos++
}

// InsertNewLine inserts a new line at the cursor position
func (is *InputState) InsertNewLine() {
	currentLine := is.lines[is.cursorLine]
	leftPart := currentLine[:is.cursorPos]
	rightPart := currentLine[is.cursorPos:]
	
	// Update current line with left part
	is.lines[is.cursorLine] = leftPart
	
	// Insert new line with right part
	newLines := make([]string, len(is.lines)+1)
	copy(newLines[:is.cursorLine+1], is.lines[:is.cursorLine+1])
	newLines[is.cursorLine+1] = rightPart
	copy(newLines[is.cursorLine+2:], is.lines[is.cursorLine+1:])
	
	is.lines = newLines
	is.cursorLine++
	is.cursorPos = 0
	is.ensureCursorVisible()
}

// Backspace removes the character before the cursor
func (is *InputState) Backspace() {
	if is.cursorPos > 0 {
		line := is.lines[is.cursorLine]
		is.lines[is.cursorLine] = line[:is.cursorPos-1] + line[is.cursorPos:]
		is.cursorPos--
	} else if is.cursorLine > 0 {
		// Join with previous line
		prevLine := is.lines[is.cursorLine-1]
		is.cursorPos = len(prevLine)
		is.lines[is.cursorLine-1] = prevLine + is.lines[is.cursorLine]
		is.lines = append(is.lines[:is.cursorLine], is.lines[is.cursorLine+1:]...)
		is.cursorLine--
		is.ensureCursorVisible()
	}
}

// InsertText inserts text at the cursor position
func (is *InputState) InsertText(text string) {
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		// Single line insert
		is.InsertChar(text)
	} else {
		// Multi-line insert
		currentLine := is.lines[is.cursorLine]
		leftPart := currentLine[:is.cursorPos]
		rightPart := currentLine[is.cursorPos:]
		
		// First line: combine with left part of current line
		lines[0] = leftPart + lines[0]
		
		// Last line: combine with right part of current line
		lastIdx := len(lines) - 1
		lastLineBeforeRightPart := lines[lastIdx]
		lines[lastIdx] = lines[lastIdx] + rightPart
		
		// Replace current line and insert new lines
		newInput := make([]string, 0, len(is.lines)+len(lines)-1)
		newInput = append(newInput, is.lines[:is.cursorLine]...)
		newInput = append(newInput, lines...)
		newInput = append(newInput, is.lines[is.cursorLine+1:]...)
		
		is.lines = newInput
		is.cursorLine += len(lines) - 1
		is.cursorPos = len(lastLineBeforeRightPart)
		is.ensureCursorVisible()
	}
}

// ensureCursorVisible ensures the cursor is within the visible scroll area
func (is *InputState) ensureCursorVisible() {
	const maxVisibleLines = 5
	
	// If cursor is above visible area, scroll up
	if is.cursorLine < is.scrollOffset {
		is.scrollOffset = is.cursorLine
	}
	
	// If cursor is below visible area, scroll down
	if is.cursorLine >= is.scrollOffset + maxVisibleLines {
		is.scrollOffset = is.cursorLine - maxVisibleLines + 1
	}
	
	// Ensure scroll offset doesn't go negative
	if is.scrollOffset < 0 {
		is.scrollOffset = 0
	}
}