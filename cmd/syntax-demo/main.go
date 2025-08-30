// Demo program showcasing the syntax highlighting system
package main

import (
	"fmt"
	"log"
	"strings"

	"gocodenow/internal/tools"
)

func main() {
	fmt.Println("🎨 Syntax Highlighting System Demo")
	fmt.Println(strings.Repeat("=", 50))
	
	// Create a syntax highlighter with default theme
	highlighter := tools.NewSyntaxHighlighter("monokai")
	
	// Demo 1: Highlight Go code
	fmt.Println("\n📝 Go Code Highlighting:")
	goCode := `package main

import "fmt"

func main() {
	message := "Hello, Syntax Highlighting!"
	fmt.Println(message)
	
	for i := 0; i < 3; i++ {
		fmt.Printf("  Iteration %d\n", i)
	}
}`
	
	highlighted, err := highlighter.HighlightCode(goCode, tools.HighlightOptions{
		Language:    "Go",
		LineNumbers: true,
		TabWidth:    4,
	})
	if err != nil {
		log.Printf("Error highlighting Go code: %v", err)
	} else {
		fmt.Println(highlighted)
	}
	
	// Demo 2: Highlight Python code with different theme
	fmt.Println("\n🐍 Python Code Highlighting (GitHub theme):")
	highlighter.SetTheme("github")
	
	pythonCode := `def fibonacci(n):
    """Generate Fibonacci sequence up to n terms."""
    a, b = 0, 1
    result = []
    
    for _ in range(n):
        result.append(a)
        a, b = b, a + b
    
    return result

# Generate first 10 Fibonacci numbers
print(fibonacci(10))`
	
	highlighted, err = highlighter.HighlightCode(pythonCode, tools.HighlightOptions{
		Language:    "Python",
		LineNumbers: false,
	})
	if err != nil {
		log.Printf("Error highlighting Python code: %v", err)
	} else {
		fmt.Println(highlighted)
	}
	
	// Demo 3: Highlight JavaScript/TypeScript
	fmt.Println("\n🌐 TypeScript Code Highlighting (Dracula theme):")
	highlighter.SetTheme("dracula")
	
	tsCode := `interface User {
    id: number;
    name: string;
    email: string;
}

class UserService {
    private users: User[] = [];
    
    async getUser(id: number): Promise<User | undefined> {
        return this.users.find(u => u.id === id);
    }
    
    addUser(user: User): void {
        this.users.push(user);
        console.log(` + "`" + `Added user: ${user.name}` + "`" + `);
    }
}`
	
	highlighted, err = highlighter.HighlightFile(tsCode, "example.ts", tools.HighlightOptions{
		LineNumbers: true,
		StartLine:   100, // Start line numbering at 100
	})
	if err != nil {
		log.Printf("Error highlighting TypeScript code: %v", err)
	} else {
		fmt.Println(highlighted)
	}
	
	// Demo 4: Highlight a diff
	fmt.Println("\n🔄 Diff Highlighting:")
	highlighter.SetTheme("monokai")
	
	diff := `--- a/main.go
+++ b/main.go
@@ -1,5 +1,6 @@
 package main
 
 import (
+    "context"
     "fmt"
     "log"
@@ -10,7 +11,7 @@ func main() {
     fmt.Println("Hello, World!")
     
-    result := processData()
+    result := processDataWithContext(context.Background())
     
     if result != nil {
         log.Println("Success!")
`
	
	highlighted, err = highlighter.HighlightDiff(diff, tools.DiffHighlightOptions{
		ShowLineNumbers: false,
	})
	if err != nil {
		log.Printf("Error highlighting diff: %v", err)
	} else {
		fmt.Println(highlighted)
	}
	
	// Demo 5: Extract and highlight code blocks from markdown
	fmt.Println("\n📄 Markdown Code Block Extraction:")
	
	markdown := `# Example Documentation

Here's how to use our API:

` + "```go" + `
client := api.NewClient("api-key")
response, err := client.Get("/users")
` + "```" + `

And in Python:

` + "```python" + `
client = APIClient("api-key")
response = client.get("/users")
` + "```"
	
	blocks, err := highlighter.ExtractCodeBlocks(markdown)
	if err != nil {
		log.Printf("Error extracting code blocks: %v", err)
	} else {
		for i, block := range blocks {
			fmt.Printf("\n  Block %d (%s):\n", i+1, block.Language)
			fmt.Println(block.Highlighted)
		}
	}
	
	// Demo 6: Show available themes
	fmt.Println("\n🎨 Available Themes:")
	themes := highlighter.GetAvailableThemes()
	themeManager := tools.NewThemeManager("monokai")
	
	// Group themes by type
	darkThemes := []string{}
	lightThemes := []string{}
	
	for _, theme := range themes {
		info := themeManager.GetThemeInfo(theme)
		if info.Dark {
			darkThemes = append(darkThemes, theme)
		} else if info.Light {
			lightThemes = append(lightThemes, theme)
		}
	}
	
	fmt.Println("  Dark Themes:")
	for _, theme := range darkThemes {
		fmt.Printf("    - %s\n", theme)
	}
	
	fmt.Println("\n  Light Themes:")
	for _, theme := range lightThemes {
		fmt.Printf("    - %s\n", theme)
	}
	
	// Demo 7: Language detection
	fmt.Println("\n🔍 Language Detection:")
	detector := tools.NewLanguageDetector()
	
	testSnippets := map[string]string{
		"Go": `func main() { fmt.Println("Hello") }`,
		"Python": `def hello(): print("Hello")`,
		"JavaScript": `const greet = () => console.log("Hello");`,
		"Java": `public static void main(String[] args) {}`,
		"Rust": `fn main() { println!("Hello"); }`,
	}
	
	for expected, code := range testSnippets {
		detected := detector.DetectLanguage(code)
		if detected == "" {
			detected = "unknown"
		}
		fmt.Printf("  Expected: %-12s Detected: %s\n", expected, detected)
	}
	
	// Demo 8: Conversation syntax highlighting
	fmt.Println("\n💬 Conversation Content Highlighting:")
	conv := tools.NewConversationSyntaxHighlighter("monokai")
	
	conversationContent := `User asked for a function to calculate factorial.

Here's the solution:

` + "```go" + `
func factorial(n int) int {
    if n <= 1 {
        return 1
    }
    return n * factorial(n-1)
}
` + "```" + `

This is a recursive implementation.`
	
	highlighted, err = conv.HighlightConversationContent(conversationContent)
	if err != nil {
		log.Printf("Error highlighting conversation: %v", err)
	} else {
		fmt.Println(highlighted)
	}
	
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("✅ Syntax Highlighting Demo Complete!")
	fmt.Println("\nFeatures demonstrated:")
	fmt.Println("  • Multi-language syntax highlighting")
	fmt.Println("  • Theme switching (20+ themes)")
	fmt.Println("  • Line numbering with custom start")
	fmt.Println("  • Diff highlighting")
	fmt.Println("  • Code block extraction from markdown")
	fmt.Println("  • Language auto-detection")
	fmt.Println("  • Conversation content enhancement")
}