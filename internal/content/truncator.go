package content

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type SmartTruncator struct{}

func NewSmartTruncator() *SmartTruncator {
	return &SmartTruncator{}
}

func (t *SmartTruncator) TruncateMarkdown(markdown string, maxLength int) string {
	if len(markdown) <= maxLength {
		return markdown
	}

	sections := t.splitBySections(markdown)
	if len(sections) == 0 {
		return t.truncateSection(markdown, maxLength)
	}
	return t.selectBestFit(sections, maxLength)
}

func (t *SmartTruncator) splitBySections(markdown string) []Section {
	lines := strings.Split(markdown, "\n")
	sections := []Section{}
	currentSection := Section{}
	inCodeBlock := false
	firstSection := true

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inCodeBlock && strings.HasPrefix(trimmedLine, "#") && !strings.HasPrefix(trimmedLine, "##") {
			if !firstSection && len(currentSection.Content) > 0 {
				sections = append(sections, currentSection)
			}
			currentSection = Section{
				Title:    trimmedLine,
				Content:  "",
				Priority: t.calculatePriority(trimmedLine),
			}
			firstSection = false
		}
		currentSection.Content += line + "\n"
	}

	if len(currentSection.Content) > 0 {
		sections = append(sections, currentSection)
	}

	return sections
}

func (t *SmartTruncator) selectBestFit(sections []Section, maxLength int) string {
	if len(sections) == 0 {
		return ""
	}

	var result strings.Builder
	remainingLength := maxLength

	for i, section := range sections {
		sectionLength := len(section.Content)

		if sectionLength <= remainingLength {
			result.WriteString(section.Content)
			remainingLength -= sectionLength
		} else if remainingLength > 100 {
			// Truncate this section to fit
			truncateAt := remainingLength - 50 // Leave room for truncation message
			if truncateAt < 100 {
				truncateAt = min(remainingLength, sectionLength)
			}
			truncated := t.truncateSection(section.Content, truncateAt)
			result.WriteString(truncated)
			if i < len(sections)-1 {
				result.WriteString("\n\n... (content truncated) ...\n")
			}
			break
		} else {
			if result.Len() > 0 {
				result.WriteString("\n\n... (remaining content omitted) ...\n")
			}
			break
		}
	}

	return strings.TrimSpace(result.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (t *SmartTruncator) truncateSection(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	truncated := t.truncateAtSentenceBoundary(content, maxLength)
	if truncated != "" {
		return truncated
	}

	truncated = t.truncateAtWordBoundary(content, maxLength)
	if truncated != "" {
		return truncated
	}

	runes := []rune(content)
	if len(runes) > maxLength {
		return string(runes[:maxLength])
	}
	return content
}

func (t *SmartTruncator) truncateAtSentenceBoundary(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	sentences := t.splitIntoSentences(content)
	var result strings.Builder

	for i, sentence := range sentences {
		if i > 0 {
			result.WriteString(" ")
		}
		if result.Len()+len(sentence) > maxLength {
			break
		}
		result.WriteString(sentence)
	}

	truncated := result.String()
	if len(truncated) > maxLength/2 {
		return truncated
	}

	return ""
}

func (t *SmartTruncator) splitIntoSentences(text string) []string {
	var sentences []string
	var current strings.Builder
	runes := []rune(text)

	for i, r := range runes {
		current.WriteRune(r)

		if t.isSentenceEnd(r) {
			// Check if it's really a sentence end (not ellipsis)
			if r == '.' && i > 0 && i < len(runes)-1 {
				// Check for ellipsis
				if runes[i-1] == '.' || (i+1 < len(runes) && runes[i+1] == '.') {
					continue
				}
			}

			// Add sentence if followed by space or at end
			if i+1 >= len(runes) || unicode.IsSpace(runes[i+1]) {
				sentences = append(sentences, strings.TrimSpace(current.String()))
				current.Reset()
				// Skip the space after sentence
				if i+1 < len(runes) && unicode.IsSpace(runes[i+1]) {
					i++
				}
			}
		}
	}

	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}

	return sentences
}

func (t *SmartTruncator) isSentenceEnd(r rune) bool {
	return r == '.' || r == '!' || r == '?' || r == '。' || r == '！' || r == '？'
}

func (t *SmartTruncator) truncateAtWordBoundary(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	truncateAt := maxLength
	for truncateAt > 0 && truncateAt < len(content) {
		r, size := utf8.DecodeRuneInString(content[truncateAt:])
		if unicode.IsSpace(r) {
			return strings.TrimSpace(content[:truncateAt])
		}
		truncateAt -= size
	}

	return ""
}

func (t *SmartTruncator) calculatePriority(title string) int {
	lowerTitle := strings.ToLower(title)

	highPriorityKeywords := []string{"summary", "overview", "introduction", "main", "content", "abstract"}
	for _, keyword := range highPriorityKeywords {
		if strings.Contains(lowerTitle, keyword) {
			return 10
		}
	}

	lowPriorityKeywords := []string{"reference", "appendix", "notes", "comments", "related", "see also"}
	for _, keyword := range lowPriorityKeywords {
		if strings.Contains(lowerTitle, keyword) {
			return 1
		}
	}

	return 5
}

type Section struct {
	Title    string
	Content  string
	Priority int
}
