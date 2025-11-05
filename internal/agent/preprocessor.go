package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type PreprocessorResult struct {
	OriginalQuery string   `json:"original_query"`
	Queries       []string `json:"queries"`
	DateCorrected bool     `json:"date_corrected"`
}

// extractQuotedPhrases extracts quoted phrases from a query
func extractQuotedPhrases(query string) []string {
	re := regexp.MustCompile(`"([^"]*)"`)
	matches := re.FindAllString(query, -1)
	return matches
}

// hasQuotedPhrases checks if a query contains quoted phrases
func hasQuotedPhrases(query string) bool {
	return strings.Contains(query, "\"")
}

// validateQuotePreservation ensures quoted phrases are preserved in processed queries
func validateQuotePreservation(original string, processed []string) bool {
	originalQuotes := extractQuotedPhrases(original)
	if len(originalQuotes) == 0 {
		return true // No quotes to preserve
	}

	// Check if all processed queries preserve at least some of the original quotes
	for _, processedQuery := range processed {
		processedQuotes := extractQuotedPhrases(processedQuery)
		if len(processedQuotes) == 0 {
			return false // Lost quotes in processing
		}

		// Verify at least one original quote is preserved
		found := false
		for _, originalQuote := range originalQuotes {
			for _, processedQuote := range processedQuotes {
				if strings.Contains(processedQuote, strings.Trim(originalQuote, "\"")) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (a *SearchAgent) preprocessQuery(ctx context.Context, query string, previousQuery *string) (*PreprocessorResult, error) {
	currentYear := time.Now().Format("2006")

	// Build context section
	contextSection := fmt.Sprintf("Query: \"%s\"", query)
	if previousQuery != nil && *previousQuery != "" {
		contextSection = fmt.Sprintf("Query: \"%s\"\nPrevious Query: \"%s\"", query, *previousQuery)
	}

	prompt := fmt.Sprintf(`You are a search query optimizer that generates strategic search queries for Google.

TASK: When given a current query and optional previous query, generate search queries that explore new aspects while avoiding previously explored concepts.

<CURRENT_CONTEXT>
Current Year: %s
%s
</CURRENT_CONTEXT>

<QUERY_NORMALIZATION>
Transform the input into effective Google search queries by:
- Converting questions to search terms (e.g., "What is X?" → "X explanation guide")
- Organizing keyword dumps into coherent searches
- Removing unnecessary words (how, what, when, etc.) unless essential
- Preserving technical terms, specific models, brands or products, and quoted phrases exactly
</QUERY_NORMALIZATION>

<INSTRUCTIONS>
For all queries:
1. First, normalize the query for Google search:
   - If it's a natural language question, extract key search terms
   - If it's a keyword dump, organize into coherent phrase
   - Keep quoted phrases, technical terms and specific brands intact

When previous_query exists:
2. Identify the domain/technology from previous query (e.g., "React hooks" → React domain)
3. Extract the NEW aspect from current query (e.g., "performance optimization")
4. Generate queries that combine: [domain context] + [new aspect]
5. AVOID repeating the exact previous query terms
6. Keep domain awareness but explore the new dimension

When no previous_query:
2. Generate comprehensive search variations
3. Add temporal markers where appropriate

IMPORTANT: Always generate EXACTLY 3 queries - no more, no less.
</INSTRUCTIONS>

<EXAMPLES>
Example 1 - With Previous Context:
Previous: "React hooks best practices 2024"
Current: "React hooks performance optimization"
Analysis: "React hooks" overlaps, "performance optimization" is new
Output: {
  "original_query": "React hooks performance optimization",
  "queries": [
    "React hooks performance optimization",
    "useMemo useCallback performance patterns 2025",
    "React rendering optimization strategies 2025"
  ],
  "date_corrected": true
}

Example 2 - With Previous Context:
Previous: "node js memory leak production"
Current: "chrome devtools heap snapshot nodejs"
Analysis: "nodejs" and "heap memory" overlaps, "chrome devtools heap snapshot" is new
Output: {
  "original_query": "chrome devtools heap snapshot nodejs",
  "queries": [
    "chrome devtools heap snapshot nodejs",
    "chrome devtools heap profiling techniques 2025",
    "memory snapshot interpretation guide nodejs"
  ],
  "date_corrected": true
}

Example 3 - No Previous Context:
Previous: null
Current: "Docker container orchestration"
Output: {
  "original_query": "Docker container orchestration",
  "queries": [
    "Docker container orchestration",
    "Docker container orchestration 2025",
    "container orchestration best practices"
  ],
  "date_corrected": true
}

Example 4 - Natural Language Question:
Previous: null
Current: "What are the best ways to optimize React performance?"
Output: {
  "original_query": "What are the best ways to optimize React performance?",
  "queries": [
    "React performance optimization techniques 2025",
    "React rendering optimization best practices",
    "React performance profiling tools guide"
  ],
  "date_corrected": true
}

Example 5 - Keyword Dump:
Previous: null
Current: "python async await concurrency parallelism"
Output: {
  "original_query": "python async await concurrency parallelism",
  "queries": [
    "python async await concurrency patterns",
    "python parallelism vs concurrency guide 2025",
    "asyncio concurrent programming best practices"
  ],
  "date_corrected": true
}
</EXAMPLES>

<TEMPORAL_RULES>
- Technical queries: Add %s for current year
- Historical queries: Preserve specific years
- Quoted phrases: Keep exactly as-is
</TEMPORAL_RULES>

Return JSON format:
{
  "original_query": "<normalized input query>",
  "queries": ["<query1>", "<query2>", "<query3>"],
  "date_corrected": true/false
}`, currentYear, contextSection, currentYear)

	response, err := a.llmClient.Complete(ctx, prompt, GeminiFlash25)
	if err != nil {
		return nil, fmt.Errorf("failed to preprocess query: %w", err)
	}

	// Clean up response - remove markdown code blocks if present
	cleanResponse := response
	if strings.HasPrefix(response, "```json") && strings.HasSuffix(response, "```") {
		cleanResponse = strings.TrimPrefix(response, "```json")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	} else if strings.HasPrefix(response, "```") && strings.HasSuffix(response, "```") {
		cleanResponse = strings.TrimPrefix(response, "```")
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
		cleanResponse = strings.TrimSpace(cleanResponse)
	}

	var result PreprocessorResult
	if err := json.Unmarshal([]byte(cleanResponse), &result); err != nil {
		// Fallback to original behavior if parsing fails
		return &PreprocessorResult{
			OriginalQuery: query,
			Queries:       []string{query},
			DateCorrected: false,
		}, nil
	}

	// Ensure we have exactly 3 queries
	if len(result.Queries) == 0 {
		result.Queries = []string{query}
	}

	// Enforce exactly 3 queries
	if len(result.Queries) > 3 {
		result.Queries = result.Queries[:3]
	} else if len(result.Queries) < 3 {
		// Pad with variations of the original query
		for len(result.Queries) < 3 {
			if len(result.Queries) == 1 {
				result.Queries = append(result.Queries, query+" 2025")
			} else {
				result.Queries = append(result.Queries, query+" best practices")
			}
		}
	}

	// Ensure original query is set
	if result.OriginalQuery == "" {
		result.OriginalQuery = query
	}

	// Validate quote preservation if original query had quotes
	if hasQuotedPhrases(query) {
		if !validateQuotePreservation(query, result.Queries) {
			// If quote preservation failed, fallback to original query
			result.Queries = []string{query}
		}
	}

	return &result, nil
}
