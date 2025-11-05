package agent

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"arguseek/internal/content"
)

type WebFetcher struct {
	httpClient *http.Client
	config     FetchConfig
	processor  *content.Processor
}


type FetchConfig struct {
	MaxRetries     int
	BackoffBase    time.Duration
	MaxBackoff     time.Duration
	TargetSources  int
	RequestTimeout time.Duration
	DialTimeout    time.Duration
	TLSTimeout     time.Duration
}

func NewWebFetcher() *WebFetcher {
	return NewWebFetcherWithPDFSupport(nil)
}

func NewWebFetcherWithPDFSupport(pdfHandler *content.DefaultPDFHandler) *WebFetcher {
	config := FetchConfig{
		MaxRetries:     3,
		BackoffBase:    100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		TargetSources:  15,
		RequestTimeout: 10 * time.Second, // Reduced from 15s to 10s
		DialTimeout:    5 * time.Second,  // Reduced from 10s to 5s for faster connection failure
		TLSTimeout:     5 * time.Second,  // Reduced from 10s to 5s for faster TLS failure
	}

	return &WebFetcher{
		httpClient: &http.Client{
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   config.DialTimeout,
					KeepAlive: 30 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   config.TLSTimeout,
				ResponseHeaderTimeout: 5 * time.Second, // Reduced from 10s to 5s for faster header failure
				ExpectContinueTimeout: 1 * time.Second,
			},
			Timeout: config.RequestTimeout,
		},
		config:    config,
		processor: func() *content.Processor {
			p := content.NewProcessor()
			if pdfHandler != nil {
				p.SetPDFHandler(pdfHandler)
			}
			return p
		}(),
	}
}

func (f *WebFetcher) FetchMultiple(ctx context.Context, urls []string) (map[string]string, error) {
	return f.FetchMultipleWithFallback(ctx, urls, nil, len(urls))
}

// FetchMultipleForResearch fetches multiple URLs with a research query context for better content extraction
func (f *WebFetcher) FetchMultipleForResearch(ctx context.Context, urls []string, researchQuery string) (map[string]string, error) {
	return f.FetchMultipleWithContext(ctx, urls, nil, len(urls), researchQuery)
}

func (f *WebFetcher) Fetch(ctx context.Context, url string, lookingFor string) (string, error) {
	return f.fetchSingleWithContext(ctx, url, lookingFor)
}

func (f *WebFetcher) FetchMultipleWithFallback(ctx context.Context, primaryURLs []string, backupURLs []string, targetCount int) (map[string]string, error) {
	return f.FetchMultipleWithContext(ctx, primaryURLs, backupURLs, targetCount, "")
}

// FetchMultipleWithContext fetches multiple URLs with a context hint for better content extraction
func (f *WebFetcher) FetchMultipleWithContext(ctx context.Context, primaryURLs []string, backupURLs []string, targetCount int, contextHint string) (map[string]string, error) {
	results := make(map[string]string)
	resultsMutex := sync.Mutex{}

	var wg sync.WaitGroup

	// Try primary URLs first - unlimited concurrency
	for _, url := range primaryURLs {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()

			// Add panic recovery for goroutine safety
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Goroutine panic recovered for URL %s: %v", targetURL, r)
					resultsMutex.Lock()
					results[targetURL] = ""
					resultsMutex.Unlock()
				}
			}()

			content, err := f.fetchSingleWithRetryAndContext(ctx, targetURL, f.config.MaxRetries, "")

			resultsMutex.Lock()
			if err == nil {
				results[targetURL] = content
			} else {
				results[targetURL] = ""
			}
			resultsMutex.Unlock()
		}(url)
	}

	wg.Wait()

	// Count successful fetches
	successCount := 0
	for _, content := range results {
		if content != "" {
			successCount++
		}
	}

	// Use backup URLs if we haven't reached target count
	if successCount < targetCount && len(backupURLs) > 0 {
		neededCount := targetCount - successCount
		backupCount := min(neededCount, len(backupURLs))

		for i := 0; i < backupCount; i++ {
			url := backupURLs[i]
			wg.Add(1)
			go func(targetURL string) {
				defer wg.Done()

				// Add panic recovery for goroutine safety
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Backup goroutine panic recovered for URL %s: %v", targetURL, r)
						resultsMutex.Lock()
						results[targetURL] = ""
						resultsMutex.Unlock()
					}
				}()

				content, err := f.fetchSingleWithRetryAndContext(ctx, targetURL, f.config.MaxRetries, "")

				resultsMutex.Lock()
				if err == nil {
					results[targetURL] = content
				} else {
					results[targetURL] = ""
				}
				resultsMutex.Unlock()
			}(url)
		}

		wg.Wait()
	}

	return results, nil
}

func (f *WebFetcher) fetchSingle(ctx context.Context, url string) (string, error) {
	return f.fetchSingleWithRetry(ctx, url, f.config.MaxRetries)
}

func (f *WebFetcher) fetchSingleWithContext(ctx context.Context, url string, lookingFor string) (string, error) {
	return f.fetchSingleWithRetryAndContext(ctx, url, f.config.MaxRetries, lookingFor)
}

func (f *WebFetcher) fetchSingleWithRetry(ctx context.Context, url string, maxRetries int) (string, error) {
	return f.fetchSingleWithRetryAndContext(ctx, url, maxRetries, "")
}

func (f *WebFetcher) fetchSingleWithRetryAndContext(ctx context.Context, url string, maxRetries int, lookingFor string) (string, error) {
	// Basic URL validation
	if url == "" || (!strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://")) {
		return "", fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff with full jitter
			baseDelay := time.Duration(1<<uint(attempt-1)) * f.config.BackoffBase
			if baseDelay > f.config.MaxBackoff {
				baseDelay = f.config.MaxBackoff
			}
			// Full jitter: random between 0 and baseDelay
			jitter := time.Duration(rand.Int63n(int64(baseDelay)))
			
			// Respect context deadline during backoff
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(jitter):
			}
		}
		
		content, err := f.fetchSingleAttempt(ctx, url, lookingFor)
		if err == nil {
			return content, nil
		}
		
		// Don't retry on non-retryable errors or context cancellation
		if !f.isRetryableError(err) || ctx.Err() != nil {
			return "", err
		}
		
		lastErr = err
	}
	return "", lastErr
}

func (f *WebFetcher) fetchSingleAttempt(ctx context.Context, url string, lookingFor string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "ArguSeek/1.0 (https://github.com/arguseek)")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // Increased to 5MB for better content processing
	if err != nil {
		return "", err
	}

	// Check content type for PDF
	contentType := resp.Header.Get("Content-Type")
	log.Printf("🔍 FETCHER DEBUG: URL=%s, Content-Type=%s, Size=%d bytes", url, contentType, len(body))
	
	if strings.Contains(strings.ToLower(contentType), "application/pdf") {
		log.Printf("📄 PDF DETECTED: %s (size: %d bytes)", url, len(body))
		// Process as PDF with looking_for parameter
		options := content.ProcessingOptions{
			MaxLength:          30000,
			PreferMainContent:  true,
			PreserveCodeBlocks: true,
			PreserveTables:     true,
			LookingFor:         lookingFor,
		}
		processedContent, err := f.processor.ProcessPDF(ctx, body, options)
		if err != nil {
			log.Printf("❌ PDF processing failed for %s: %v", url, err)
			// Return a user-friendly error message for corrupted PDFs
			if strings.Contains(err.Error(), "corrupted") || strings.Contains(err.Error(), "unsupported") {
				return "", fmt.Errorf("unable to process PDF: document may be corrupted or unsupported")
			}
			// Return the original error for other types of failures
			return "", err
		}
		log.Printf("✅ PDF processed successfully: %s", url)
		return processedContent.Markdown, nil
	}

	// Process as HTML
	html := string(body)
	
	// Use the content processor with unified configuration
	options := content.ProcessingOptions{
		MaxLength:          30000,
		PreferMainContent:  true,
		PreserveCodeBlocks: true,
		PreserveTables:     true,
		LookingFor:         lookingFor,
	}
	
	processedContent, err := f.processor.ProcessHTMLWithTimeout(html, options, 5*time.Second)
	if err != nil {
		// Content processor includes its own fallback mechanism
		log.Printf("Content processing failed for %s: %v", url, err)
		return "", err
	}

	// Log processing metrics for monitoring
	if processedContent.ProcessingInfo.ProcessingTimeMS > 0 {
		log.Printf("Content processing metrics for %s: time=%dms, strategy=%s, original=%d, processed=%d, fallback=%v",
			url, processedContent.ProcessingInfo.ProcessingTimeMS,
			processedContent.ProcessingInfo.ExtractionStrategy,
			processedContent.ProcessingInfo.OriginalLength,
			processedContent.ProcessingInfo.ProcessedLength,
			processedContent.ProcessingInfo.UsedFallback)
	}

	return processedContent.Markdown, nil
}

func (f *WebFetcher) isRetryableError(err error) bool {
	// Check for 5xx status codes, timeouts, network errors
	errStr := err.Error()
	return strings.Contains(errStr, "HTTP 5") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "temporary failure")
}


