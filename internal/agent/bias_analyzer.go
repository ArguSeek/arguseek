package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type BiasAnalyzer struct {
	llmClient LLMClient
}

func NewBiasAnalyzer(llmClient LLMClient) *BiasAnalyzer {
	return &BiasAnalyzer{
		llmClient: llmClient,
	}
}

func (b *BiasAnalyzer) AnalyzeBias(ctx context.Context, fetchedContent map[string]string, biasTruncation int) BiasAnalysisResult {
	if len(fetchedContent) == 0 {
		return BiasAnalysisResult{
			BiasCategory: "none",
		}
	}

	prompt := b.buildBiasAnalysisPrompt(fetchedContent, biasTruncation)

	// Use temperature 0.2 for consistent yet slightly varied analysis
	temp := 0.2
	response, err := b.llmClient.CompleteWithTemp(ctx, prompt, GeminiFlash25, &temp)
	if err != nil {
		// Return safe default on error
		return BiasAnalysisResult{
			BiasCategory: "none",
			Explanation:  fmt.Sprintf("Analysis failed: %v", err),
		}
	}

	return b.parseResponse(response)
}

func (b *BiasAnalyzer) buildBiasAnalysisPrompt(fetchedContent map[string]string, biasTruncation int) string {
	var xmlBuilder strings.Builder
	xmlBuilder.WriteString("<SOURCES>\n")

	for url, content := range fetchedContent {
		if content != "" {
			// Truncate content and escape XML
			escapedContent := escapeXML(truncateContent(content, biasTruncation))
			escapedURL := escapeXML(url)
			xmlBuilder.WriteString(fmt.Sprintf(`  <SOURCE url="%s">%s</SOURCE>%s`,
				escapedURL, escapedContent, "\n"))
		}
	}
	xmlBuilder.WriteString("</SOURCES>\n\n")

	return fmt.Sprintf(`You are analyzing web search results for potential bias patterns in content.

%s

Your task is to classify these sources into EXACTLY ONE category based on patterns of coordination, bias, or missing perspectives:

**Categories:**
- "none": Diverse viewpoints with balanced coverage, different perspectives represented
- "seo_campaign": Coordinated content with suspiciously similar messaging, phrasing, or structure across sources
- "product_promotion": Multiple sources promoting the same product/service with minimal criticism
- "unanimous_technical": All sources agree on a technical approach without presenting alternatives or trade-offs
- "one_sided": Missing critical perspectives, drawbacks, or counterarguments

**Few-shot examples:**

Example 1 (seo_campaign):
Sources all use phrases like "best solution for" and "industry-leading" with identical benefit lists.
Result: {"category": "seo_campaign", "indicators": ["identical_benefit_lists", "repeated_marketing_phrases"], "counter_query": "independent reviews problems with [topic]"}

Example 2 (product_promotion):
Multiple sources recommend the same specific product, mentioning only price as a concern.
Result: {"category": "product_promotion", "indicators": ["same_product_recommended", "minimal_criticism"], "counter_query": "[product name] real world problems user complaints"}

Example 3 (unanimous_technical):
All sources agree React hooks are always the best approach without mentioning class components or alternatives.
Result: {"category": "unanimous_technical", "indicators": ["no_alternatives_discussed", "unanimous_recommendation"], "counter_query": "React hooks disadvantages performance issues alternatives"}

Example 4 (one_sided):
All sources praise a new technology without mentioning limitations, costs, or implementation challenges.
Result: {"category": "one_sided", "indicators": ["no_drawbacks_mentioned", "only_positive_aspects"], "counter_query": "[technology] limitations challenges real world problems"}

Example 5 (none):
Sources present different viewpoints, some favor option A with reasoning, others favor option B, includes criticism and alternatives.
Result: {"category": "none", "indicators": ["diverse_viewpoints", "balanced_coverage"], "counter_query": ""}

**Analysis Guidelines:**
- Look for linguistic similarity across sources (repeated phrases, identical structure)
- Check if all sources unanimously recommend without presenting alternatives
- Identify missing critical perspectives (no drawbacks, limitations, or criticism)
- Consider if sources feel coordinated rather than independently written
- Sources can agree and still be legitimate if they present reasoning and acknowledge trade-offs

**CRITICAL INSTRUCTIONS:**
- If category is "none", set counter_query to empty string ""
- Only suggest counter_query for actual bias patterns (seo_campaign, product_promotion, unanimous_technical, one_sided)
- Counter-queries should explore the missing perspectives or criticism absent from the sources

Output ONLY valid JSON in this exact format:
{
  "category": "[exactly one of: none, seo_campaign, product_promotion, unanimous_technical, one_sided]",
  "indicators": ["specific patterns you observed"],
  "counter_query": "[query to explore missing perspectives, or empty string if category is none]"
}`, xmlBuilder.String())
}

func (b *BiasAnalyzer) parseResponse(response string) BiasAnalysisResult {
	// Clean the response - remove any markdown formatting
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	// Parse JSON response
	var jsonResponse struct {
		Category     string   `json:"category"`
		Indicators   []string `json:"indicators"`
		CounterQuery string   `json:"counter_query"`
	}

	if err := json.Unmarshal([]byte(response), &jsonResponse); err != nil {
		// Return safe default on parse error
		return BiasAnalysisResult{
			BiasCategory: "none",
			Explanation:  fmt.Sprintf("Failed to parse response: %v", err),
		}
	}

	// Validate category
	validCategories := map[string]bool{
		"none":                true,
		"seo_campaign":        true,
		"product_promotion":   true,
		"unanimous_technical": true,
		"one_sided":           true,
	}

	category := strings.ToLower(strings.TrimSpace(jsonResponse.Category))
	if !validCategories[category] {
		category = "none"
	}

	// Ensure counter_query is empty for "none" category
	counterQuery := strings.TrimSpace(jsonResponse.CounterQuery)
	if category == "none" {
		counterQuery = ""
	}

	return BiasAnalysisResult{
		BiasCategory:   category,
		BiasIndicators: jsonResponse.Indicators,
		CounterQuery:   counterQuery,
		Explanation:    fmt.Sprintf("Classified as %s with %d indicators", category, len(jsonResponse.Indicators)),
	}
}
