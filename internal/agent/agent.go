package agent

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"arguseek/internal/content"
)

type SearchAgent struct {
	config       Config
	searchClient SearchClient
	llmClient    LLMClient
	fetcher      ContentFetcher
	biasAnalyzer *BiasAnalyzer
}

type Config struct {
	GoogleAPIKey       string
	GoogleCSEID        string
	GeminiConfig       GeminiConfig
	MaxResultsPerQuery int
}

type SearchResult struct {
	Query   string   `json:"query"`
	Answer  string   `json:"answer"`
	Sources []string `json:"sources"`
}

func NewSearchAgent(ctx context.Context, config Config) (*SearchAgent, error) {
	// Google API credentials are optional for testing
	if config.GoogleAPIKey == "" {
		config.GoogleAPIKey = "test-key"
	}
	if config.GoogleCSEID == "" {
		config.GoogleCSEID = "test-cse"
	}

	if config.MaxResultsPerQuery == 0 {
		config.MaxResultsPerQuery = 10
	}

	// Create the unified Gemini client (supports both Gemini API and Vertex AI)
	llmClient, err := NewGeminiClient(ctx, config.GeminiConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	searchClient := NewGoogleSearchClient(config.GoogleAPIKey, config.GoogleCSEID)

	// Create PDF processing infrastructure using the shared genai client
	pdfClient := NewPDFClient(llmClient.client)
	pdfChunker := content.NewPDFChunker()
	pdfHandler := content.NewPDFHandler(pdfClient, pdfChunker)

	fetcher := NewWebFetcherWithPDFSupport(pdfHandler)
	biasAnalyzer := NewBiasAnalyzer(llmClient)

	return &SearchAgent{
		config:       config,
		searchClient: searchClient,
		llmClient:    llmClient,
		fetcher:      fetcher,
		biasAnalyzer: biasAnalyzer,
	}, nil
}

// NewSearchAgentWithDependencies creates a SearchAgent with injected dependencies for testing
func NewSearchAgentWithDependencies(config Config, searchClient SearchClient, llmClient LLMClient, fetcher ContentFetcher) *SearchAgent {
	if config.MaxResultsPerQuery == 0 {
		config.MaxResultsPerQuery = 10
	}

	biasAnalyzer := NewBiasAnalyzer(llmClient)

	return &SearchAgent{
		config:       config,
		searchClient: searchClient,
		llmClient:    llmClient,
		fetcher:      fetcher,
		biasAnalyzer: biasAnalyzer,
	}
}

// TestPreprocessor exposes the preprocessor for testing
func (a *SearchAgent) TestPreprocessor(ctx context.Context, query string, previousQuery *string) (*PreprocessorResult, error) {
	return a.preprocessQuery(ctx, query, previousQuery)
}

func (a *SearchAgent) ResearchIteratively(ctx context.Context, query string, previousQuery *string) (string, error) {
	if strings.TrimSpace(query) == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	// Use preprocessor to get optimized queries
	preprocessResult, err := a.preprocessQuery(ctx, query, previousQuery)
	if err != nil {
		// Fallback to original query on error
		preprocessResult = &PreprocessorResult{
			OriginalQuery: query,
			Queries:       []string{query},
			DateCorrected: false,
		}
	}

	queries := preprocessResult.Queries

	searchTasks := make([]SearchTask, len(queries))
	var wg sync.WaitGroup

	for i, q := range queries {
		wg.Add(1)
		go func(idx int, searchQuery string) {
			defer wg.Done()

			results, err := a.searchClient.Search(ctx, searchQuery, a.config.MaxResultsPerQuery)
			if err != nil {
				searchTasks[idx] = SearchTask{
					Query: searchQuery,
					Error: err,
				}
				return
			}

			searchTasks[idx] = SearchTask{
				Query:   searchQuery,
				Results: results,
			}
		}(i, q)
	}

	wg.Wait()

	urlToIndex := make(map[string]int)
	var allResults []GoogleSearchResult
	var searchErrors []string
	globalIndex := 0

	for _, task := range searchTasks {
		if task.Error != nil {
			searchErrors = append(searchErrors, fmt.Sprintf("Query '%s' failed: %v", task.Query, task.Error))
		} else if task.Results != nil {
			for _, result := range task.Results {
				normalizedURL := normalizeURL(result.Link)
				if existingIndex, exists := urlToIndex[normalizedURL]; !exists || globalIndex < existingIndex {
					urlToIndex[normalizedURL] = globalIndex
				}
				globalIndex++
				allResults = append(allResults, result)
			}
		}
	}

	// Sort URLs by index to preserve Google's ranking
	type urlIndexPair struct {
		url   string
		index int
	}

	urlPairs := make([]urlIndexPair, 0, len(urlToIndex))
	for url, index := range urlToIndex {
		urlPairs = append(urlPairs, urlIndexPair{url: url, index: index})
	}

	sort.Slice(urlPairs, func(i, j int) bool {
		return urlPairs[i].index < urlPairs[j].index
	})

	// Take up to 30 unique URLs (15 primary + 15 backup for fallback mechanism)
	maxURLs := min(30, len(urlPairs))

	allURLs := make([]string, maxURLs)
	for i := 0; i < maxURLs; i++ {
		allURLs[i] = urlPairs[i].url
	}

	if len(allURLs) == 0 {
		if len(searchErrors) > 0 {
			return "", fmt.Errorf("search failed: %s", strings.Join(searchErrors, "; "))
		}
		return "No search results found", nil
	}

	// Content fetching with fallback mechanism
	primaryCount := min(15, len(allURLs))
	primaryURLs := allURLs[:primaryCount]
	var backupURLs []string

	if len(allURLs) > primaryCount {
		backupURLs = allURLs[primaryCount:]
	}

	targetSourceCount := 12 // Target: get at least 12 successful sources

	// Use fallback mechanism: try primary URLs first, then backups if needed
	fetchedContent, err := a.fetcher.FetchMultipleWithFallback(ctx, primaryURLs, backupURLs, targetSourceCount)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content: %w", err)
	}

	// Run bias analysis and synthesis concurrently
	biasResultChan := make(chan BiasAnalysisResult, 1)
	go func() {
		result := a.biasAnalyzer.AnalyzeBias(ctx, fetchedContent)
		biasResultChan <- result
	}()

	// Run synthesis in parallel with bias analysis
	type synthesisResult struct {
		answer string
		err    error
	}
	synthesisResultChan := make(chan synthesisResult, 1)
	go func() {
		// Use default bias result for initial synthesis
		tempBiasResult := BiasAnalysisResult{BiasCategory: "none"}
		answer, err := a.synthesizeAnswer(ctx, preprocessResult.OriginalQuery, previousQuery, allResults, fetchedContent, tempBiasResult)
		synthesisResultChan <- synthesisResult{answer: answer, err: err}
	}()

	// Wait for both to complete
	var biasResult BiasAnalysisResult
	select {
	case biasResult = <-biasResultChan:
	case <-ctx.Done():
		biasResult = BiasAnalysisResult{BiasCategory: "none"}
	}

	synthesisRes := <-synthesisResultChan
	if synthesisRes.err != nil {
		return "", fmt.Errorf("failed to synthesize answer: %w", synthesisRes.err)
	}

	// If bias was detected, update the footer in the synthesized answer
	answer := synthesisRes.answer
	if biasResult.BiasCategory != "none" && biasResult.CounterQuery != "" {
		// Find and replace the standard footer with bias warning
		categoryLabel := map[string]string{
			"seo_campaign":        "coordinated messaging",
			"product_promotion":   "promotional content",
			"unanimous_technical": "one-sided coverage",
			"one_sided":           "missing perspectives",
		}[biasResult.BiasCategory]

		if categoryLabel == "" {
			categoryLabel = "potential bias"
		}

		biasFooter := fmt.Sprintf("\n\n---\n⚠️ **Pattern detected (%s):** Sources show %s\n🔍 **Get balanced view:** Use `research_iteratively(query: \"%s\", previous_query: \"%s\")`\n📄 **Verify sources:** Use `fetch_url(url: \"[URL from above]\")` to examine details",
			biasResult.BiasCategory, categoryLabel, biasResult.CounterQuery, preprocessResult.OriginalQuery)

		// Replace standard footer if it exists
		standardFooterIdx := strings.Index(answer, "\n\n---\n🔍 **Continue exploring:**")
		if standardFooterIdx > 0 {
			answer = answer[:standardFooterIdx] + biasFooter
		} else if !strings.Contains(answer, "⚠️ **Pattern detected") {
			answer = answer + biasFooter
		}
	}

	return answer, nil
}

// FetchURL fetches and extracts content from a single URL with optional context
func (a *SearchAgent) FetchURL(ctx context.Context, urlStr string, lookingFor string) (string, error) {
	if strings.TrimSpace(urlStr) == "" {
		return "", fmt.Errorf("URL cannot be empty")
	}

	// Fetch content using the fetcher
	content, err := a.fetcher.Fetch(ctx, urlStr, lookingFor)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}

	if content == "" {
		return "", fmt.Errorf("no content fetched from URL")
	}

	// Content is already processed and truncated by the fetcher
	truncatedContent := content

	// Build extraction prompt
	var prompt string
	if lookingFor != "" {
		// Query-focused extraction using Gemini best practices
		prompt = fmt.Sprintf(`You are extracting specific information from a webpage.

URL: %s
LOOKING FOR: %s

EXTRACTION APPROACH:
1. First, understand what the user needs: %s
2. Scan the content for relevant sections about this topic
3. Extract ONLY information directly related to what they're looking for
4. If the exact information isn't present, clearly state: "This page doesn't contain information about %s"
5. Then note what related information IS available

FORMATTING RULES:
- Use clean paragraphs without bullet points or asterisks
- For lists: Use simple dashes (-) only when absolutely necessary  
- Section headers: Use ## sparingly, only for major topics
- Keep formatting minimal and readable

RESPONSE STRUCTURE:
- Start with the most relevant information
- Present data in the format that best suits the content type (tables for comparisons, steps for instructions)
- Be precise and actionable

Content:
%s`, urlStr, lookingFor, lookingFor, lookingFor, truncatedContent)
	} else {
		// Keep existing general extraction prompt unchanged
		prompt = fmt.Sprintf(`Extract information from this webpage following STRICT formatting rules.

URL: %s

CRITICAL FORMATTING REQUIREMENTS:
- Use clean paragraphs without bullet points or asterisks
- For lists: Use simple dashes (-) only when absolutely necessary
- Section headers: Use ## sparingly, only for major topics
- NO asterisks (*) for emphasis or lists
- Keep formatting minimal and readable

EXAMPLE OF CORRECT FORMAT:
## Main Topic
Clear paragraph explaining the topic without bullets or asterisks.

Key information presented in clean sentences. When listing must-have items:
- First item (only if necessary)
- Second item (keep brief)

Another paragraph with detailed information.

Instructions:
1. Read through the content carefully
2. Identify the main topics and key points
3. Present information in clean, readable paragraphs
4. Focus on factual, actionable information
5. Cite specific details when relevant

VALIDATION BEFORE RESPONDING:
- Ensure NO lines start with * or **
- Verify minimal use of formatting
- Check that response uses clean paragraphs

Content:
%s`, urlStr, truncatedContent)
	}

	// Use LLM to extract content
	response, err := a.llmClient.Complete(ctx, prompt, GeminiFlash25)
	if err != nil {
		return "", fmt.Errorf("failed to extract content: %w", err)
	}

	// Format response with suffix prompt
	formattedResponse := fmt.Sprintf(`**Content extracted from:** %s

%s

---
📄 **Read more sources:** Use `+"`"+`fetch_url(url: "[URL]")`+"`"+` for additional pages
🔍 **Broader research:** Use `+"`"+`research_iteratively(query: "[topic]")`+"`"+` to find more sources`,
		urlStr,
		response)

	return formattedResponse, nil
}

// normalizeURL normalizes URLs for consistent deduplication
func normalizeURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // Return original if parsing fails
	}

	// Remove fragment
	u.Fragment = ""

	// Remove trailing slash from path
	if u.Path != "/" && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}

	// Convert to lowercase hostname
	u.Host = strings.ToLower(u.Host)

	return u.String()
}

// detectQueryIntent analyzes the query to determine its type
func (a *SearchAgent) detectQueryIntent(query string) string {
	lowerQuery := strings.ToLower(query)

	// Implementation queries
	if strings.Contains(lowerQuery, "how to") || strings.Contains(lowerQuery, "implement") ||
		strings.Contains(lowerQuery, "create") || strings.Contains(lowerQuery, "build") ||
		strings.Contains(lowerQuery, "setup") || strings.Contains(lowerQuery, "configure") ||
		strings.Contains(lowerQuery, "install") || strings.Contains(lowerQuery, "deploy") {
		return "implementation"
	}

	// Troubleshooting queries
	if strings.Contains(lowerQuery, "error") || strings.Contains(lowerQuery, "fix") ||
		strings.Contains(lowerQuery, "solve") || strings.Contains(lowerQuery, "debug") ||
		strings.Contains(lowerQuery, "issue") || strings.Contains(lowerQuery, "problem") ||
		strings.Contains(lowerQuery, "not working") || strings.Contains(lowerQuery, "fails") {
		return "troubleshooting"
	}

	// Comparison queries
	if strings.Contains(lowerQuery, "vs") || strings.Contains(lowerQuery, "versus") ||
		strings.Contains(lowerQuery, "compare") || strings.Contains(lowerQuery, "difference") ||
		strings.Contains(lowerQuery, "better") || strings.Contains(lowerQuery, "which") {
		return "comparison"
	}

	// Conceptual queries
	if strings.Contains(lowerQuery, "what is") || strings.Contains(lowerQuery, "explain") ||
		strings.Contains(lowerQuery, "define") || strings.Contains(lowerQuery, "concept") ||
		strings.Contains(lowerQuery, "theory") || strings.Contains(lowerQuery, "why") {
		return "conceptual"
	}

	return "general"
}

// detectSourceType analyzes a URL to determine its source type
func (a *SearchAgent) detectSourceType(url string) string {
	lowerURL := strings.ToLower(url)

	if strings.Contains(lowerURL, "github.com") {
		if strings.Contains(lowerURL, "/issues/") {
			return "github_issue"
		}
		return "github"
	}

	if strings.Contains(lowerURL, "stackoverflow.com") {
		return "stackoverflow"
	}

	if strings.Contains(lowerURL, "docs.") || strings.Contains(lowerURL, "documentation") {
		return "documentation"
	}

	if strings.Contains(lowerURL, "reddit.com") {
		return "reddit"
	}

	if strings.Contains(lowerURL, "medium.com") || strings.Contains(lowerURL, "dev.to") {
		return "blog"
	}

	return "general"
}

func (a *SearchAgent) synthesizeAnswer(ctx context.Context, query string, previousQuery *string, results []GoogleSearchResult, fetchedContent map[string]string, biasResult BiasAnalysisResult) (string, error) {
	// Detect query intent
	queryIntent := a.detectQueryIntent(query)

	// Build XML-structured prompt with ALL content
	var xmlBuilder strings.Builder
	xmlBuilder.WriteString("<DOCUMENTS>\n")

	urlToID := make(map[string]string)
	sourceTypes := make(map[string]string)

	for url, content := range fetchedContent {
		if content != "" {
			// Detect source type
			sourceType := a.detectSourceType(url)
			sourceTypes[url] = sourceType

			// CRITICAL: Escape XML special characters
			// Using 30,000 chars per document (well within 1M token limit)
			escapedContent := escapeXML(truncateContent(content, 30000))
			escapedURL := escapeXML(url)

			xmlBuilder.WriteString(fmt.Sprintf(`  <SOURCE url="%s" type="%s">%s</SOURCE>%s`,
				escapedURL, sourceType, escapedContent, "\n"))
			urlToID[url] = url
		}
	}

	if len(urlToID) == 0 {
		return "Unable to fetch content from search results", nil
	}

	xmlBuilder.WriteString("</DOCUMENTS>\n\n")

	// Build query-specific guidance
	var queryGuidance string
	switch queryIntent {
	case "implementation":
		queryGuidance = "When applicable, focus on implementation steps and practical examples."
	case "troubleshooting":
		queryGuidance = "When addressing problems, focus on solutions and root causes."
	case "comparison":
		queryGuidance = "When comparing options, present multiple perspectives and trade-offs."
	case "conceptual":
		queryGuidance = "When explaining concepts, provide clear explanations with examples."
	default:
		queryGuidance = "Provide a comprehensive response addressing all aspects of the query."
	}

	// Build context note if this is a follow-up query
	var contextNote string
	if previousQuery != nil && *previousQuery != "" {
		contextNote = fmt.Sprintf(`
Note: This is a follow-up question. The previous query was: "%s"
The current query must be interpreted in the context of this previous topic.

`, escapeXML(*previousQuery))
	}

	// Build flexible, natural synthesis prompt with enforced structure
	prompt := fmt.Sprintf(`You are a helpful research assistant synthesizing information from web sources.
%s
%s<QUERY>%s</QUERY>

Context: These sources were gathered from Google Search and represent the most relevant results available.

CRITICAL FORMATTING REQUIREMENT: Your response MUST include two mandatory sections:
1. The synthesized answer with inline citations
2. A complete "Sources Used:" section listing all cited sources

Example of CORRECT sequential numbering:
"The technology uses advanced algorithms [1] combined with machine learning [2]. 
Recent studies [3] confirm these findings, while earlier work [1] laid the foundation."

Sources Used:
[1] URL1 - Description
[2] URL2 - Description  
[3] URL3 - Description

INCORRECT (has gaps): [1], [3], [7] 
CORRECT (sequential): [1], [2], [3]

Guidelines:
- Before writing, mentally number all sources you'll cite starting from 1
- Synthesize information from all provided sources to answer the query
- Number sources sequentially as you cite them [1], [2], [3] in order of first citation
- Source Quality Assessment:
  * GitHub issues: Check if URL contains '/issues/' - if closed, treat as potentially outdated
  * StackOverflow: Questions describe what DOES NOT work - focus only on answers, especially accepted ones. Disregard any negatively-rated answers
  * Documentation sources are typically most authoritative
  * Blog posts represent individual perspectives
- If sources disagree, present multiple viewpoints rather than forcing consensus
- Adapt your response style to the actual query content and sources found
- %s

Source awareness:
- Documentation sources are typically authoritative
- GitHub issues may contain outdated information if closed - look for issue status indicators
- StackOverflow: The question section describes problems/failures - prioritize accepted answers and positively-rated solutions only
- Blog posts represent individual perspectives

MANDATORY OUTPUT FORMAT:
1. Your synthesized answer with [citations]
2. A blank line (use an actual empty line in Markdown - no HTML tags)
3. **Sources Used:**
4. List each cited source as: [1] URL - Brief description

CRITICAL MARKDOWN FORMATTING:
- Use only Markdown syntax (**, *, #, lists, links, etc.)
- For blank lines: use literal empty lines in Markdown
- NEVER mix HTML tags (<br>, <p>, <div>) with Markdown
- The blank line before 'Sources Used:' should be an actual empty line
- Keep all formatting consistent with standard Markdown

VALIDATION CHECKPOINT - MANDATORY SEQUENTIAL NUMBERING:
Step 1: Count how many unique sources you will cite
Step 2: Assign numbers 1 through N to these sources in citation order
Step 3: Verify NO GAPS - if you cite 5 sources, use [1][2][3][4][5]
Step 4: Double-check that Sources Used section lists EXACTLY these numbers
CRITICAL: If gaps exist (e.g., [1][3][5]), you MUST renumber to [1][2][3]

Your response MUST end with the complete Sources Used section. Do not omit this section under any circumstances.

Keep your response comprehensive but concise, focusing on what's most relevant to the query.

Begin your response now, ensuring you follow the complete format:`, contextNote, xmlBuilder.String(), escapeXML(query), queryGuidance)

	response, err := a.llmClient.Complete(ctx, prompt, GeminiFlash25)
	if err != nil {
		return "", err
	}

	// Generate unified footer based on bias detection
	var footerInstruction string
	if biasResult.BiasCategory != "none" && biasResult.CounterQuery != "" {
		// Bias detected - optimized, action-oriented footer
		categoryLabel := map[string]string{
			"seo_campaign":        "coordinated messaging",
			"product_promotion":   "promotional content",
			"unanimous_technical": "one-sided coverage",
			"one_sided":           "missing perspectives",
		}[biasResult.BiasCategory]

		if categoryLabel == "" {
			categoryLabel = "potential bias"
		}

		footerInstruction = fmt.Sprintf(`

---
⚠️ **Pattern detected (%s):** Sources show %s
🔍 **Get balanced view:** Use `+"`research_iteratively(query: \"%s\", previous_query: \"%s\")`"+`
📄 **Verify sources:** Use `+"`fetch_url(url: \"[URL from above]\")`"+` to examine details`,
			biasResult.BiasCategory, categoryLabel, biasResult.CounterQuery, query)
	} else {
		// Standard footer - optimized wording
		footerInstruction = "\n\n---\n🔍 **Continue exploring:** Use `research_iteratively(query: \"[follow-up]\", previous_query: \"" + query + "\")`\n📄 **Read sources:** Use `fetch_url(url: \"[URL from above]\")` to examine details"
	}
	response = response + footerInstruction

	return response, nil
}

type SearchTask struct {
	Query   string
	Results []GoogleSearchResult
	Error   error
}

func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// escapeXML escapes special characters for XML content
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
