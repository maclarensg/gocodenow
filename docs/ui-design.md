# gocodenow TUI Design

## Main Interface Layout

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ (1) gocodenow v1.0 | Model: gpt-4 | Status: Ready                           │
├─────────────────────────────────────────────────────────────────────────────┤
│                          CONVERSATION HISTORY (2)                           │
│                                                                             │
│ ┌─ Conversation Block 1 ────────────────────────────────── [14:32] ───────┐ │
│ │ ▶ User: Help me implement a login system                                │ │
│ │   Assistant: I'll help you create a login system...                     │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ ┌─ Conversation Block 2 ────────────────────────────────── [14:45] ───────┐ │
│ │ ▼ User: Fix the authentication bug in user.js                           │ │
│ │   ├─ Request: Fix auth bug                                               │ │
│ │   ├─ Files Modified: user.js, auth.js                                   │ │
│ │   ├─ Tools Used: Read, Edit, Bash                                       │ │
│ │   └─ Response: Fixed authentication by updating token validation...     │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│ ┌─ Conversation Block 3 ────────────────────────────────── [15:01] ───────┐ │
│ │ ▶ User: Add error handling to the API endpoints                         │ │
│ │   Assistant: I'll add comprehensive error handling...                   │ │
│ └─────────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│                              [Scroll for more]                             │
│                                                                             │
├─────────────────────────────────────────────────────────────────────────────┤
│ > Type your message here... (3)                                             │
│                                                               [Send: Enter] │
└─────────────────────────────────────────────────────────────────────────────┘
```
> (1) Header will be FIXED at top, always show, adjusts to screen width
> (2) Conversation area is SCROLLABLE, shows as many conversation blocks as possible in remaining space
> (3) Input area is FIXED at bottom, adjusts to screen width

## Layout Behavior Rules:

### Fixed Positioning:
- **Header**: Always stuck at top of screen from program start
- **Footer/Input**: Always stuck at bottom of screen 
- **Middle**: Conversation area uses ALL remaining space between header and footer

### Scrolling Behavior:
- **Show maximum conversations**: Fill available space with as many conversation blocks as possible
- **Scroll only when needed**: When conversations exceed available space, enable scrolling
- **Auto-scroll**: New messages automatically scroll to show latest conversation
- **Manual scroll**: Users can scroll up/down to view conversation history

### Responsive Design:
- **Screen resize**: All components adjust width immediately
- **Height recalculation**: Conversation area recalculates available space on resize

## Expanded Conversation Block

When a block is selected and expanded (▼):

```
┌─ Conversation Block 2 (EXPANDED) ─────────────────────── [14:45] ───────────┐
│ ▼ User: Fix the authentication bug in user.js                              │
│                                                                             │
│   ┌─ REQUEST DETAILS ──────────────────────────────────────────────────────┐ │
│   │ Original Message: "Fix the authentication bug in user.js"             │ │
│   │ Context: /home/project/src/                                           │ │
│   └───────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│   ┌─ ASSISTANT ACTIONS ─────────────────────────────────────────────────────┐ │
│   │ [1] Read: user.js:1-50                                                │ │
│   │ [2] Read: auth.js:1-30                                                │ │
│   │ [3] Edit: user.js:23 (Fixed token validation)                         │ │
│   │ [4] Bash: npm test (Ran tests to verify fix)                          │ │
│   └───────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
│   ┌─ RESPONSE ──────────────────────────────────────────────────────────────┐ │
│   │ I've fixed the authentication bug by updating the token validation    │ │
│   │ logic in user.js:23. The issue was in the JWT verification where...   │ │
│   │                                                                        │ │
│   │ Changes made:                                                          │ │
│   │ • Updated validateToken() function                                     │ │
│   │ • Added proper error handling                                          │ │
│   │ • Tests now pass ✓                                                     │ │
│   └───────────────────────────────────────────────────────────────────────┘ │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Key Design Elements

### Navigation
- **Arrow Keys**: Navigate between conversation blocks
- **Enter**: Expand/collapse selected block
- **Tab**: Focus on input prompt
- **Ctrl+C**: Exit application

### Block States
- **Collapsed (▶)**: Shows summary of conversation
- **Expanded (▼)**: Shows full details including tools used and complete response
- **Active**: Highlighted border when selected

### Status Indicators
- Model name and connection status in header
- Timestamp for each conversation
- Tool usage indicators in expanded view
- File modification indicators

### Input Area
- Bottom-fixed input prompt
- Send with Enter key
- Multi-line support with Ctrl+Enter (NOTE: Use Ctrl+Enter instead of Shift+Enter for better cross-terminal compatibility)
- Command suggestions with Tab completion