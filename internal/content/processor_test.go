package content

import (
	"strings"
	"testing"
	"time"
)

func TestNewProcessor(t *testing.T) {
	p := NewProcessor()
	if p == nil {
		t.Fatal("expected processor to be non-nil")
	}
	if p.extractor == nil {
		t.Fatal("expected extractor to be initialized")
	}
	if p.truncator == nil {
		t.Fatal("expected truncator to be initialized")
	}
}

func TestProcessHTML_SimpleHTML(t *testing.T) {
	p := NewProcessor()
	html := `
	<html>
		<head><title>Test Page</title></head>
		<body>
			<h1>Hello World</h1>
			<p>This is a test paragraph.</p>
		</body>
	</html>
	`

	options := ProcessingOptions{
		MaxLength:         1000,
		PreferMainContent: true,
	}

	result, err := p.ProcessHTML(html, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExtractedTitle != "Test Page" {
		t.Errorf("expected title 'Test Page', got '%s'", result.ExtractedTitle)
	}

	if !strings.Contains(result.Markdown, "Hello World") {
		t.Error("expected markdown to contain 'Hello World'")
	}

	if !strings.Contains(result.Markdown, "This is a test paragraph") {
		t.Error("expected markdown to contain paragraph text")
	}
}

func TestProcessHTML_WithMainElement(t *testing.T) {
	p := NewProcessor()
	html := `
	<html>
		<body>
			<nav>Navigation content</nav>
			<main>
				<h1>Main Content</h1>
				<p>This is the main content area.</p>
			</main>
			<footer>Footer content</footer>
		</body>
	</html>
	`

	options := ProcessingOptions{
		MaxLength:         1000,
		PreferMainContent: true,
	}

	result, err := p.ProcessHTML(html, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.ProcessingInfo.MainContentFound {
		t.Error("expected main content to be found")
	}

	if result.ProcessingInfo.ExtractionStrategy != "main_element" {
		t.Errorf("expected extraction strategy 'main_element', got '%s'", result.ProcessingInfo.ExtractionStrategy)
	}

	if strings.Contains(result.Markdown, "Navigation content") {
		t.Error("markdown should not contain navigation content")
	}

	if strings.Contains(result.Markdown, "Footer content") {
		t.Error("markdown should not contain footer content")
	}

	if !strings.Contains(result.Markdown, "Main Content") {
		t.Error("expected markdown to contain main content")
	}
}

func TestProcessHTML_CodeBlockPreservation(t *testing.T) {
	p := NewProcessor()
	html := `
	<html>
		<body>
			<article>
				<h1>Code Example</h1>
				<p>Here's some code:</p>
				<pre><code class="language-go">
func main() {
    fmt.Println("Hello, World!")
}
				</code></pre>
			</article>
		</body>
	</html>
	`

	options := ProcessingOptions{
		MaxLength:          1000,
		PreferMainContent:  true,
		PreserveCodeBlocks: true,
	}

	result, err := p.ProcessHTML(html, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result.Markdown, "```go") {
		t.Error("expected markdown to contain Go code block marker")
	}

	if !strings.Contains(result.Markdown, "func main()") {
		t.Error("expected markdown to contain code content")
	}

	if !strings.Contains(result.Markdown, "fmt.Println") {
		t.Error("expected markdown to contain println statement")
	}
}

func TestProcessHTML_TablePreservation(t *testing.T) {
	p := NewProcessor()
	html := `
	<html>
		<body>
			<main>
				<h1>Data Table</h1>
				<table>
					<thead>
						<tr>
							<th>Name</th>
							<th>Value</th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<td>Item 1</td>
							<td>100</td>
						</tr>
						<tr>
							<td>Item 2</td>
							<td>200</td>
						</tr>
					</tbody>
				</table>
			</main>
		</body>
	</html>
	`

	options := ProcessingOptions{
		MaxLength:      1000,
		PreserveTables: true,
	}

	result, err := p.ProcessHTML(html, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that table data is preserved in some form
	if !strings.Contains(result.Markdown, "Item 1") || !strings.Contains(result.Markdown, "100") {
		t.Error("expected markdown to contain table data")
	}

	if !strings.Contains(result.Markdown, "Name") || !strings.Contains(result.Markdown, "Value") {
		t.Error("expected markdown to contain table headers")
	}
}

func TestProcessHTML_Truncation(t *testing.T) {
	p := NewProcessor()

	// Generate long content
	var longContent strings.Builder
	longContent.WriteString("<html><body><main>")
	for i := 0; i < 100; i++ {
		longContent.WriteString("<p>This is paragraph number ")
		longContent.WriteString(strings.Repeat("very ", 20))
		longContent.WriteString("long.</p>")
	}
	longContent.WriteString("</main></body></html>")

	options := ProcessingOptions{
		MaxLength:         500,
		PreferMainContent: true,
	}

	result, err := p.ProcessHTML(longContent.String(), options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Markdown) > 600 { // Allow some overhead for truncation message
		t.Errorf("expected markdown length to be around %d, got %d", options.MaxLength, len(result.Markdown))
	}

	if result.ProcessingInfo.ProcessedLength >= result.ProcessingInfo.OriginalLength {
		t.Error("expected processed length to be less than original")
	}
}

func TestProcessHTML_Fallback(t *testing.T) {
	p := NewProcessor()
	// Test with plain text that looks like content but isn't HTML
	plainText := "This is plain text content, not HTML"

	options := ProcessingOptions{
		MaxLength: 1000,
	}

	result, err := p.ProcessHTML(plainText, options)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The important thing is that content is preserved
	if !strings.Contains(result.Markdown, "plain text content") {
		t.Error("expected content to be preserved")
	}

	// Check that it went through some processing
	if result.ProcessingInfo.ProcessingTimeMS < 0 {
		t.Error("expected valid processing time")
	}
}

func TestProcessHTMLWithTimeout(t *testing.T) {
	p := NewProcessor()
	html := `<html><body><h1>Test</h1></body></html>`

	options := ProcessingOptions{
		MaxLength: 1000,
	}

	// Test with reasonable timeout
	result, err := p.ProcessHTMLWithTimeout(html, options, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error with reasonable timeout: %v", err)
	}

	if !strings.Contains(result.Markdown, "Test") {
		t.Error("expected result to contain content")
	}

	// Test with very short timeout (might not always trigger timeout, but tests the path)
	_, _ = p.ProcessHTMLWithTimeout(html, options, 1*time.Nanosecond)
	// We don't assert error here as it's timing-dependent
}

func TestDetectContentType(t *testing.T) {
	p := NewProcessor()

	tests := []struct {
		html         string
		expectedType string
	}{
		{
			html:         `<html><body><article><h1>Blog Post</h1></article></body></html>`,
			expectedType: "article",
		},
		{
			html:         `<html><body><pre><code>code1</code></pre><pre><code>code2</code></pre><pre><code>code3</code></pre><pre><code>code4</code></pre><pre><code>code5</code></pre><pre><code>code6</code></pre></body></html>`,
			expectedType: "documentation",
		},
		{
			html:         `<html><body><div>Simple content</div></body></html>`,
			expectedType: "general",
		},
	}

	for _, test := range tests {
		options := ProcessingOptions{MaxLength: 1000}
		result, err := p.ProcessHTML(test.html, options)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			continue
		}

		if result.ContentType != test.expectedType {
			t.Errorf("expected content type '%s', got '%s'", test.expectedType, result.ContentType)
		}
	}
}
