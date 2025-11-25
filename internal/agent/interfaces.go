package agent

import "context"

// SearchClient abstracts search functionality
type SearchClient interface {
	Search(ctx context.Context, query string, numResults int) ([]GoogleSearchResult, error)
}

// LLMClient abstracts language model functionality
type LLMClient interface {
	Complete(ctx context.Context, prompt string, model GeminiModel) (string, error)
	CompleteWithTemp(ctx context.Context, prompt string, model GeminiModel, temperature *float64) (string, error)
}

// ContentFetcher abstracts web content fetching
type ContentFetcher interface {
	FetchMultiple(ctx context.Context, urls []string) (map[string]string, error)
	FetchMultipleForResearch(ctx context.Context, urls []string, researchQuery string) (map[string]string, error)
	FetchMultipleWithFallback(ctx context.Context, primaryURLs []string, backupURLs []string, targetCount int) (map[string]string, error)
	Fetch(ctx context.Context, url string, lookingFor string) (string, error)
}

// BiasAnalysisResult represents the result of bias analysis on fetched content
type BiasAnalysisResult struct {
	BiasCategory   string   // "none", "seo_campaign", "product_promotion", "unanimous_technical", "one_sided"
	BiasIndicators []string // Specific patterns detected
	CounterQuery   string   // Suggested follow-up query (empty if category is "none")
	Explanation    string   // Brief explanation of detected patterns
}

// Interface compliance verification
var (
	_ SearchClient   = (*GoogleSearchClient)(nil)
	_ LLMClient      = (*GeminiClient)(nil)
	_ ContentFetcher = (*WebFetcher)(nil)
)
