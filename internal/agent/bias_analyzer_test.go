package agent

import (
	"context"
	"strings"
	"testing"
)

// MockLLMClient for testing bias analyzer
type MockLLMClient struct {
	response string
	err      error
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string, model GeminiModel) (string, error) {
	return m.response, m.err
}

func (m *MockLLMClient) CompleteWithTemp(ctx context.Context, prompt string, model GeminiModel, temperature *float64) (string, error) {
	return m.response, m.err
}

func TestBiasAnalyzer_AnalyzeBias_None(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "none", "indicators": ["diverse_viewpoints", "balanced_coverage"], "counter_query": ""}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://example1.com": "Article discussing pros and cons of the approach",
		"https://example2.com": "Different perspective with alternative solutions",
		"https://example3.com": "Critical analysis showing limitations",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "none" {
		t.Errorf("Expected category 'none', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery != "" {
		t.Errorf("Expected empty counter_query for 'none' category, got '%s'", result.CounterQuery)
	}
}

func TestBiasAnalyzer_AnalyzeBias_SEOCampaign(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "seo_campaign", "indicators": ["identical_phrasing", "repeated_marketing_phrases"], "counter_query": "independent reviews problems with React hooks"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://blog1.com": "Best solution for React development using hooks",
		"https://blog2.com": "Industry-leading approach with React hooks best solution",
		"https://blog3.com": "Best solution for modern React with hooks industry-leading",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "seo_campaign" {
		t.Errorf("Expected category 'seo_campaign', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery == "" {
		t.Error("Expected counter_query for 'seo_campaign' category, got empty string")
	}

	if len(result.BiasIndicators) == 0 {
		t.Error("Expected bias indicators, got none")
	}
}

func TestBiasAnalyzer_AnalyzeBias_ProductPromotion(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "product_promotion", "indicators": ["same_product_recommended", "minimal_criticism"], "counter_query": "Next.js real world problems user complaints"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://review1.com": "Next.js is amazing for all projects. Only downside is learning curve.",
		"https://review2.com": "Next.js solved all our problems. Minor issue with build times.",
		"https://review3.com": "Next.js is perfect solution. Small price concern but worth it.",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "product_promotion" {
		t.Errorf("Expected category 'product_promotion', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery == "" {
		t.Error("Expected counter_query for 'product_promotion' category, got empty string")
	}
}

func TestBiasAnalyzer_AnalyzeBias_UnanimousTechnical(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "unanimous_technical", "indicators": ["no_alternatives_discussed", "unanimous_recommendation"], "counter_query": "microservices disadvantages monolith alternatives"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://tech1.com": "Microservices architecture is always the best approach for scalability",
		"https://tech2.com": "Every modern application should use microservices for optimal performance",
		"https://tech3.com": "Microservices solve all scalability problems and should be used everywhere",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "unanimous_technical" {
		t.Errorf("Expected category 'unanimous_technical', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery == "" {
		t.Error("Expected counter_query for 'unanimous_technical' category, got empty string")
	}
}

func TestBiasAnalyzer_AnalyzeBias_OneSided(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "one_sided", "indicators": ["no_drawbacks_mentioned", "only_positive_aspects"], "counter_query": "AI limitations challenges real world problems"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://ai1.com": "AI will revolutionize everything with amazing benefits",
		"https://ai2.com": "AI technology brings incredible advantages to all industries",
		"https://ai3.com": "AI is the perfect solution with unlimited potential for growth",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "one_sided" {
		t.Errorf("Expected category 'one_sided', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery == "" {
		t.Error("Expected counter_query for 'one_sided' category, got empty string")
	}
}

func TestBiasAnalyzer_AnalyzeBias_EmptyContent(t *testing.T) {
	mockLLM := &MockLLMClient{}
	analyzer := NewBiasAnalyzer(mockLLM)

	result := analyzer.AnalyzeBias(context.Background(), map[string]string{}, 10000)

	if result.BiasCategory != "none" {
		t.Errorf("Expected category 'none' for empty content, got '%s'", result.BiasCategory)
	}
}

func TestBiasAnalyzer_AnalyzeBias_ParseError(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: "invalid json response",
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://example.com": "Some content",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "none" {
		t.Errorf("Expected category 'none' on parse error, got '%s'", result.BiasCategory)
	}

	if !containsString(result.Explanation, "Failed to parse") {
		t.Errorf("Expected explanation to mention parse failure, got '%s'", result.Explanation)
	}
}

func TestBiasAnalyzer_AnalyzeBias_InvalidCategory(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "invalid_category", "indicators": [], "counter_query": "test query"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://example.com": "Some content",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "none" {
		t.Errorf("Expected category 'none' for invalid category, got '%s'", result.BiasCategory)
	}

	if result.CounterQuery != "" {
		t.Errorf("Expected empty counter_query when category falls back to 'none', got '%s'", result.CounterQuery)
	}
}

func TestBiasAnalyzer_AnalyzeBias_NoneOverridesCounterQuery(t *testing.T) {
	mockLLM := &MockLLMClient{
		response: `{"category": "none", "indicators": [], "counter_query": "this should be ignored"}`,
	}

	analyzer := NewBiasAnalyzer(mockLLM)

	fetchedContent := map[string]string{
		"https://example.com": "Balanced content with multiple perspectives",
	}

	result := analyzer.AnalyzeBias(context.Background(), fetchedContent, 10000)

	if result.BiasCategory != "none" {
		t.Errorf("Expected category 'none', got '%s'", result.BiasCategory)
	}

	if result.CounterQuery != "" {
		t.Errorf("Expected counter_query to be overridden to empty for 'none' category, got '%s'", result.CounterQuery)
	}
}

// Helper function
func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > len(needle) &&
				(haystack[:len(needle)] == needle ||
					haystack[len(haystack)-len(needle):] == needle ||
					strings.Contains(haystack, needle)))
}
