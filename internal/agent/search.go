package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type GoogleSearchClient struct {
	apiKey       string
	searchEngine string
	httpClient   *http.Client
}

type GoogleSearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

type googleSearchResponse struct {
	Items []GoogleSearchResult `json:"items"`
}

func NewGoogleSearchClient(apiKey, searchEngine string) *GoogleSearchClient {
	return &GoogleSearchClient{
		apiKey:       apiKey,
		searchEngine: searchEngine,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *GoogleSearchClient) Search(ctx context.Context, query string, numResults int) ([]GoogleSearchResult, error) {
	if numResults > 10 {
		numResults = 10
	}

	searchURL := fmt.Sprintf(
		"https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s&num=%d",
		c.apiKey,
		c.searchEngine,
		url.QueryEscape(query),
		numResults,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var searchResp googleSearchResponse
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (body: %s)", err, string(body))
	}

	return searchResp.Items, nil
}
