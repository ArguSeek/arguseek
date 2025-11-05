package content

import (
	"strings"
	"testing"
)

func TestNewSmartTruncator(t *testing.T) {
	tr := NewSmartTruncator()
	if tr == nil {
		t.Fatal("expected truncator to be non-nil")
	}
}

func TestTruncateMarkdown_ShortContent(t *testing.T) {
	tr := NewSmartTruncator()
	
	content := "# Title\n\nThis is a short paragraph."
	result := tr.TruncateMarkdown(content, 1000)
	
	if result != content {
		t.Error("short content should not be truncated")
	}
}

func TestTruncateMarkdown_LongContent(t *testing.T) {
	tr := NewSmartTruncator()
	
	var content strings.Builder
	content.WriteString("# Main Title\n\n")
	content.WriteString("This is the introduction paragraph.\n\n")
	
	// Add multiple sections
	for i := 0; i < 10; i++ {
		content.WriteString("## Section ")
		content.WriteString(string(rune('A' + i)))
		content.WriteString("\n\n")
		content.WriteString(strings.Repeat("This is content for this section. ", 50))
		content.WriteString("\n\n")
	}
	
	result := tr.TruncateMarkdown(content.String(), 500)
	
	if len(result) > 600 { // Allow some overhead for truncation message
		t.Errorf("expected truncated length around 500, got %d", len(result))
	}
	
	if !strings.Contains(result, "Main Title") {
		t.Error("expected to preserve main title")
	}
	
	if strings.Contains(result, "Section J") {
		t.Error("should not contain last section")
	}
}

func TestTruncateMarkdown_CodeBlockHandling(t *testing.T) {
	tr := NewSmartTruncator()
	
	content := `# Code Example

Here's some text before code.

` + "```go" + `
func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

More text after code block.`

	result := tr.TruncateMarkdown(content, 1000)
	
	if !strings.Contains(result, "```go") {
		t.Error("expected code block to be preserved")
	}
	
	if !strings.Contains(result, "func main()") {
		t.Error("expected code content to be preserved")
	}
}

func TestSplitBySections(t *testing.T) {
	tr := NewSmartTruncator()
	
	content := `# Title 1

Content for section 1.

## Subtitle

Content for subsection.

# Title 2

Content for section 2.`

	sections := tr.splitBySections(content)
	
	if len(sections) != 2 {
		t.Errorf("expected 2 top-level sections, got %d", len(sections))
	}
	
	if !strings.Contains(sections[0].Title, "Title 1") {
		t.Error("expected first section to have Title 1")
	}
	
	if !strings.Contains(sections[0].Content, "Subtitle") {
		t.Error("expected first section to contain subsection")
	}
	
	if !strings.Contains(sections[1].Title, "Title 2") {
		t.Error("expected second section to have Title 2")
	}
}

func TestCalculatePriority(t *testing.T) {
	tr := NewSmartTruncator()
	
	tests := []struct {
		title            string
		expectedPriority int
	}{
		{"# Introduction", 10},
		{"# Summary", 10},
		{"# Overview", 10},
		{"# Main Content", 10},
		{"# References", 1},
		{"# Appendix", 1},
		{"# See Also", 1},
		{"# Random Title", 5},
	}
	
	for _, test := range tests {
		priority := tr.calculatePriority(test.title)
		if priority != test.expectedPriority {
			t.Errorf("title '%s': expected priority %d, got %d", 
				test.title, test.expectedPriority, priority)
		}
	}
}

func TestTruncateAtSentenceBoundary(t *testing.T) {
	tr := NewSmartTruncator()
	
	content := "This is the first sentence. This is the second sentence. This is the third sentence."
	
	result := tr.truncateAtSentenceBoundary(content, 50)
	
	if result != "This is the first sentence. " {
		t.Errorf("expected truncation at first sentence boundary, got: %s", result)
	}
}

func TestTruncateAtWordBoundary(t *testing.T) {
	tr := NewSmartTruncator()
	
	content := "This is a long sentence with many words that needs truncation"
	
	result := tr.truncateAtWordBoundary(content, 25)
	
	if !strings.HasSuffix(result, "sentence") {
		t.Errorf("expected truncation at word boundary, got: %s", result)
	}
	
	if strings.Contains(result, "with") {
		t.Error("truncated too much content")
	}
}

func TestSplitIntoSentences(t *testing.T) {
	tr := NewSmartTruncator()
	
	tests := []struct {
		text              string
		expectedSentences int
	}{
		{
			text:              "First sentence. Second sentence! Third sentence?",
			expectedSentences: 3,
		},
		{
			text:              "One sentence without ending",
			expectedSentences: 1,
		},
		{
			text:              "Chinese sentence。 Japanese sentence！ Another one？",
			expectedSentences: 3,
		},
		{
			text:              "Sentence with... ellipsis. Another one.",
			expectedSentences: 2,
		},
	}
	
	for _, test := range tests {
		sentences := tr.splitIntoSentences(test.text)
		if len(sentences) != test.expectedSentences {
			t.Errorf("text '%s': expected %d sentences, got %d", 
				test.text, test.expectedSentences, len(sentences))
		}
	}
}

func TestSelectBestFit(t *testing.T) {
	tr := NewSmartTruncator()
	
	sections := []Section{
		{
			Title:    "# Important",
			Content:  strings.Repeat("Important content. ", 20),
			Priority: 10,
		},
		{
			Title:    "# Details", 
			Content:  strings.Repeat("Detailed content. ", 50),
			Priority: 5,
		},
		{
			Title:    "# References",
			Content:  strings.Repeat("Reference content. ", 30),
			Priority: 1,
		},
	}
	
	result := tr.selectBestFit(sections, 300)
	
	if !strings.Contains(result, "Important content") {
		t.Error("expected to include high priority content")
	}
	
	if strings.Contains(result, "Reference content") {
		t.Error("should not include low priority content when space is limited")
	}
	
	if !strings.Contains(result, "truncated") && !strings.Contains(result, "omitted") {
		t.Error("expected truncation indicators")
	}
}