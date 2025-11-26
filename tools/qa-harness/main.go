// qa-harness.go
//
// ArguSeek Comprehensive Quality Assurance Test Harness
// =====================================================
//
// This is a permanent test harness for validating the ArguSeek research_iteratively service
// across both development and production environments. It provides comprehensive coverage
// of functionality, edge cases, performance, and content quality validation.
//
// Configuration:
//   Set environment variables for your ArguSeek instances:
//     export ARGUSEEK_DEV_URL="https://your-dev-instance.example.com"
//     export ARGUSEEK_PROD_URL="https://your-prod-instance.example.com"
//
//   For local testing, set the required API keys:
//     export GOOGLE_API_KEY="your-google-api-key"
//     export GOOGLE_CSE_ID="your-google-cse-id"
//     export GEMINI_API_KEY="your-gemini-api-key" # Optional
//
// Usage:
//   go run qa-harness.go [local|dev|prod|compare]
//
// Examples:
//   go run qa-harness.go local  # Auto-start local server and run tests
//   go run qa-harness.go dev    # Run against development environment
//   go run qa-harness.go prod   # Run against production environment
//
// The test harness will:
//   - Validate environment configuration and health
//   - Execute comprehensive test scenarios
//   - Perform concurrent load testing (10 simultaneous queries)
//   - Validate citation numbering and content quality
//   - Generate detailed reports with actionable insights
//   - Fail fast on critical errors while continuing non-critical tests

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	// Test harness version
	VERSION = "1.0.0"

	// Environment configurations
	// Set these via environment variables or command line:
	// DEV_URL:  export ARGUSEEK_DEV_URL="https://your-dev-instance.example.com"
	// PROD_URL: export ARGUSEEK_PROD_URL="https://your-prod-instance.example.com"
	DEV_URL  = "" // Set via ARGUSEEK_DEV_URL environment variable
	PROD_URL = "" // Set via ARGUSEEK_PROD_URL environment variable

	// Test configuration
	CONCURRENT_QUERIES = 2
	REQUEST_TIMEOUT    = 90 * time.Second

	// Local server configuration
	SERVER_STARTUP_TIMEOUT  = 30 * time.Second
	SERVER_SHUTDOWN_TIMEOUT = 10 * time.Second
	HEALTH_CHECK_INTERVAL   = 500 * time.Millisecond
)

// Environment represents a test environment configuration
type Environment struct {
	Name      string
	BaseURL   string
	HealthURL string
}

// MCPRequest represents an MCP JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// MCPResponse represents an MCP JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPContentBlock represents a content block in MCP tool response
type MCPContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPToolResult represents the MCP tool call result structure
type MCPToolResult struct {
	Content []MCPContentBlock `json:"content"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolCallParams represents parameters for tools/call
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// TestCase represents a single test scenario
type TestCase struct {
	Name              string
	Query             string
	PreviousQuery     *string
	ExpectedBehavior  string
	ValidationRules   []ValidationRule
	Critical          bool
	Concurrent        bool
	IsFetchURL        bool
	FetchURL          string
	ExpectError       bool // NEW: Flag for negative tests that should produce errors
	ExpectedErrorCode int  // NEW: Expected MCP error code for negative tests
}

// ValidationRule represents a validation check
type ValidationRule struct {
	Type        string // "citation", "content", "performance", "error"
	Description string
	Validator   func(result TestResult) ValidationResult
}

// ValidationResult represents the outcome of a validation
type ValidationResult struct {
	Passed  bool
	Message string
}

// TestResult represents the result of a single test
type TestResult struct {
	TestCase  TestCase
	Response  string
	Error     error
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
}

// ConcurrentTestResult represents results from concurrent testing
type ConcurrentTestResult struct {
	TotalTime    time.Duration `json:"total_time"`
	ErrorCount   int           `json:"error_count"`
	SuccessCount int           `json:"success_count"`
	SuccessRate  float64       `json:"success_rate"`
}

// TestReport represents the comprehensive test report
type TestReport struct {
	Environment          string                `json:"environment"`
	Version              string                `json:"version"`
	StartTime            time.Time             `json:"start_time"`
	EndTime              time.Time             `json:"end_time"`
	TotalTests           int                   `json:"total_tests"`
	Passed               int                   `json:"passed"`
	Failed               int                   `json:"failed"`
	SuccessRate          float64               `json:"success_rate"`
	TotalDuration        time.Duration         `json:"total_duration"`
	AverageDuration      time.Duration         `json:"average_duration"`
	ConcurrentTestResult *ConcurrentTestResult `json:"concurrent_test_result,omitempty"`
	FailedTests          []FailedTest          `json:"failed_tests,omitempty"`
	PerformanceMetrics   PerformanceMetrics    `json:"performance_metrics"`
}

// FailedTest represents details of a failed test
type FailedTest struct {
	Name             string   `json:"name"`
	Error            string   `json:"error"`
	ValidationErrors []string `json:"validation_errors,omitempty"`
}

// PerformanceMetrics represents performance statistics
type PerformanceMetrics struct {
	FastestQuery time.Duration `json:"fastest_query"`
	SlowestQuery time.Duration `json:"slowest_query"`
	P50          time.Duration `json:"p50"`
	P90          time.Duration `json:"p90"`
	P99          time.Duration `json:"p99"`
}

func main() {
	log.Printf("ArguSeek Comprehensive QA Test Harness v%s", VERSION)
	log.Println("===========================================")

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run qa-harness.go [local|dev|prod|compare]")
	}

	command := strings.ToLower(os.Args[1])

	if command == "compare" {
		// Run pipeline comparison mode
		log.Println("Pipeline comparison mode - validating new content processor")
		runContentProcessorValidation()
		return
	}

	// Handle local mode with server lifecycle management
	if command == "local" {
		log.Println("Local mode - starting server automatically")

		// Start local server
		port, cmd, err := startLocalServer()
		if err != nil {
			log.Fatalf("Failed to start local server: %v", err)
		}

		// Ensure server is always stopped on exit
		defer func() {
			if err := stopLocalServer(cmd); err != nil {
				log.Printf("Warning: Error stopping server: %v", err)
			}
		}()

		// Wait for server to be ready
		if err := waitForServerReady(port); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}

		// Create environment with dynamic URL
		env := Environment{
			Name:      "local",
			BaseURL:   fmt.Sprintf("http://localhost:%d", port),
			HealthURL: fmt.Sprintf("http://localhost:%d/health", port),
		}

		log.Printf("Testing against %s environment", strings.ToUpper(env.Name))
		log.Printf("Base URL: %s", env.BaseURL)
		log.Println()

		// Run comprehensive test suite
		report := runComprehensiveTests(env)

		// Generate and display report
		displayReport(report)
		saveReport(env, report)
		return
	}

	// Normal test mode for dev/prod
	env := getEnvironment(command)

	log.Printf("Testing against %s environment", strings.ToUpper(env.Name))
	log.Printf("Base URL: %s", env.BaseURL)
	log.Println()

	// Run comprehensive test suite
	report := runComprehensiveTests(env)

	// Generate and display report
	displayReport(report)
	saveReport(env, report)
}

func getEnvironment(name string) Environment {
	var baseURL string
	var envName string

	// Note: "local" mode does not use this function - it constructs Environment
	// inline after starting the server (see main() function)
	switch name {
	case "dev":
		envName = "development"
		// Load from environment variable, fallback to constant
		baseURL = os.Getenv("ARGUSEEK_DEV_URL")
		if baseURL == "" {
			baseURL = DEV_URL
		}
		if baseURL == "" {
			log.Fatal("ERROR: ARGUSEEK_DEV_URL environment variable not set. Example: export ARGUSEEK_DEV_URL='https://your-dev-instance.example.com'")
		}
	case "prod":
		envName = "production"
		// Load from environment variable, fallback to constant
		baseURL = os.Getenv("ARGUSEEK_PROD_URL")
		if baseURL == "" {
			baseURL = PROD_URL
		}
		if baseURL == "" {
			log.Fatal("ERROR: ARGUSEEK_PROD_URL environment variable not set. Example: export ARGUSEEK_PROD_URL='https://your-prod-instance.example.com'")
		}
	default:
		log.Fatalf("Unknown environment: %s. Use 'local', 'dev', or 'prod'", name)
		return Environment{}
	}

	return Environment{
		Name:      envName,
		BaseURL:   baseURL,
		HealthURL: baseURL + "/health",
	}
}

func runComprehensiveTests(env Environment) TestReport {
	report := TestReport{
		Environment: env.Name,
		Version:     VERSION,
		StartTime:   time.Now(),
	}

	// Phase 1: Environment validation
	log.Println("=== Phase 1: Environment Validation ===")
	if !validateEnvironment(env) {
		log.Fatal("CRITICAL: Environment validation failed. Cannot proceed.")
	}
	log.Println("✓ Environment validation passed")
	log.Println()

	// Phase 2: Build test cases
	testCases := buildTestCases()

	// Phase 3: Run sequential tests
	log.Println("=== Phase 2: Sequential Test Execution ===")
	sequentialResults := runSequentialTests(env, testCases)

	// Phase 4: Run concurrent tests
	log.Println("\n=== Phase 3: Concurrent Test Execution ===")
	concurrentResult := runConcurrentTests(env)
	report.ConcurrentTestResult = &concurrentResult

	// Phase 5: Analyze results
	report.EndTime = time.Now()
	report.TotalDuration = report.EndTime.Sub(report.StartTime)
	analyzeResults(&report, sequentialResults)

	return report
}

func validateEnvironment(env Environment) bool {
	log.Printf("Checking health endpoint: %s", env.HealthURL)

	resp, err := http.Get(env.HealthURL)
	if err != nil {
		log.Printf("ERROR: Failed to reach health endpoint: %v", err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Printf("ERROR: Health check returned status %d", resp.StatusCode)
		return false
	}

	log.Println("✓ Health check passed")
	return true
}

func buildTestCases() []TestCase {
	return []TestCase{
		// Basic functionality tests
		{
			Name:             "Basic Single Query",
			Query:            "What are the best practices for React hooks?",
			ExpectedBehavior: "Should return comprehensive results about React hooks best practices",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Response should contain React hooks information",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "React") && strings.Contains(r.Response, "hooks") {
							return ValidationResult{Passed: true, Message: "Content validation passed"}
						}
						return ValidationResult{Passed: false, Message: "Response missing React hooks content"}
					},
				},
				{
					Type:        "citation",
					Description: "Citations should be sequentially numbered",
					Validator:   validateCitations,
				},
			},
			Critical: true,
		},
		{
			Name:             "Follow-up Query with Context",
			Query:            "What about performance optimization?",
			PreviousQuery:    stringPtr("What are the best practices for React hooks?"),
			ExpectedBehavior: "Should maintain context from previous query about React hooks",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Response should relate to React performance",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "performance") || strings.Contains(r.Response, "optimization") {
							return ValidationResult{Passed: true, Message: "Context maintained"}
						}
						return ValidationResult{Passed: false, Message: "Context not maintained from previous query"}
					},
				},
			},
		},

		// Quote handling tests
		{
			Name:             "Query with Exact Phrase Quotes",
			Query:            `"DuckDB performance tuning"`,
			ExpectedBehavior: "Should return 'no results' when exact phrase not found on web",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Should indicate no results found for exact phrase",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "No search results") || strings.Contains(r.Response, "no results") {
							return ValidationResult{Passed: true, Message: "Correctly returned no results for exact phrase"}
						}
						return ValidationResult{Passed: false, Message: "Expected 'no results' for exact phrase match"}
					},
				},
			},
		},
		{
			Name:             "Follow-up After Quoted Query",
			Query:            "What alternatives exist?",
			PreviousQuery:    stringPtr(`"DuckDB performance tuning"`),
			ExpectedBehavior: "Should maintain context even after quoted query",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Should relate to DuckDB or performance",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(strings.ToLower(r.Response), "duckdb") ||
							strings.Contains(strings.ToLower(r.Response), "database") ||
							strings.Contains(strings.ToLower(r.Response), "performance") {
							return ValidationResult{Passed: true, Message: "Context maintained after quoted query"}
						}
						return ValidationResult{Passed: false, Message: "Lost context after quoted query"}
					},
				},
			},
		},

		// Edge case tests
		{
			Name:             "Overly Restrictive Query",
			Query:            `"extremely specific nonexistent technical term 2024"`,
			ExpectedBehavior: "Should gracefully handle no results",
			ValidationRules: []ValidationRule{
				{
					Type:        "error",
					Description: "Should not error on no results",
					Validator: func(r TestResult) ValidationResult {
						if r.Error == nil {
							return ValidationResult{Passed: true, Message: "Handled no results gracefully"}
						}
						return ValidationResult{Passed: false, Message: fmt.Sprintf("Error on no results: %v", r.Error)}
					},
				},
			},
		},
		{
			Name:             "Time-Sensitive Query",
			Query:            "latest AI developments today",
			ExpectedBehavior: "Should handle time-sensitive queries",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Should return AI-related content",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(strings.ToLower(r.Response), "ai") ||
							strings.Contains(strings.ToLower(r.Response), "artificial intelligence") {
							return ValidationResult{Passed: true, Message: "Time-sensitive query handled"}
						}
						return ValidationResult{Passed: false, Message: "Failed to handle time-sensitive query"}
					},
				},
			},
		},
		{
			Name:             "GitHub Issues Query",
			Query:            "DuckDB segmentation fault crash GitHub",
			ExpectedBehavior: "Should find and properly handle GitHub issues",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Should mention GitHub or issues",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "GitHub") || strings.Contains(r.Response, "issue") {
							return ValidationResult{Passed: true, Message: "GitHub content found"}
						}
						return ValidationResult{Passed: false, Message: "No GitHub content found"}
					},
				},
			},
		},
		// Fetch URL tool tests
		{
			Name:             "Fetch URL with Context",
			Query:            "",
			IsFetchURL:       true,
			FetchURL:         "https://docs.python.org/3/library/asyncio.html",
			ExpectedBehavior: "Should fetch and extract specific asyncio information",
			ValidationRules: []ValidationRule{
				{
					Type:        "structure",
					Description: "Should have fetch_url response structure",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "**Content extracted from:**") {
							return ValidationResult{Passed: true, Message: "Proper fetch_url response structure"}
						}
						return ValidationResult{Passed: false, Message: "Missing fetch_url response structure"}
					},
				},
				{
					Type:        "content",
					Description: "Should contain asyncio-related content",
					Validator: func(r TestResult) ValidationResult {
						lower := strings.ToLower(r.Response)
						if strings.Contains(lower, "asyncio") || strings.Contains(lower, "event loop") ||
							strings.Contains(lower, "coroutine") {
							return ValidationResult{Passed: true, Message: "Asyncio content found"}
						}
						return ValidationResult{Passed: false, Message: "No asyncio content found"}
					},
				},
			},
		},
		{
			Name:             "Fetch URL without Context",
			Query:            "",
			IsFetchURL:       true,
			FetchURL:         "https://www.rust-lang.org/",
			ExpectedBehavior: "Should fetch and extract content successfully",
			ValidationRules: []ValidationRule{
				{
					Type:        "structure",
					Description: "Should have proper fetch_url response structure",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "**Content extracted from:**") {
							return ValidationResult{Passed: true, Message: "Proper fetch_url response structure"}
						}
						return ValidationResult{Passed: false, Message: "Missing fetch_url response structure"}
					},
				},
				{
					Type:        "content",
					Description: "Should contain Rust-related content",
					Validator: func(r TestResult) ValidationResult {
						lower := strings.ToLower(r.Response)
						if strings.Contains(lower, "rust") {
							return ValidationResult{Passed: true, Message: "Rust content found"}
						}
						return ValidationResult{Passed: false, Message: "No Rust content found"}
					},
				},
			},
		},
		{
			Name:              "Fetch URL Invalid URL",
			Query:             "",
			IsFetchURL:        true,
			FetchURL:          "not-a-valid-url",
			ExpectedBehavior:  "Should return error for invalid URL",
			ExpectError:       true,   // NEW: This test expects an error
			ExpectedErrorCode: -32602, // NEW: Expected JSON-RPC invalid params error code
			ValidationRules: []ValidationRule{
				{
					Type:        "error",
					Description: "Should return appropriate error for invalid URL",
					Validator: func(r TestResult) ValidationResult {
						// For negative tests, we'll handle error validation in the main logic
						// This rule now validates the error message content
						if r.Error != nil && strings.Contains(r.Error.Error(), "Invalid URL") {
							return ValidationResult{Passed: true, Message: "Correct error message returned"}
						}
						if r.Error != nil {
							return ValidationResult{Passed: true, Message: "Error returned as expected"}
						}
						return ValidationResult{Passed: false, Message: "No error returned for invalid URL"}
					},
				},
			},
		},

		// PDF processing tests
		{
			Name:             "PDF Fetch with Token Limit Validation",
			Query:            "",
			IsFetchURL:       true,
			FetchURL:         "https://arxiv.org/pdf/2506.15655",
			ExpectedBehavior: "Should fetch PDF and summarize within 30K token limit",
			ValidationRules: []ValidationRule{
				{
					Type:        "structure",
					Description: "Should have fetch_url response structure",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "**Content extracted from:**") {
							return ValidationResult{Passed: true, Message: "Proper fetch_url response structure"}
						}
						return ValidationResult{Passed: false, Message: "Missing fetch_url response structure"}
					},
				},
				{
					Type:        "content",
					Description: "Should contain cAST or chunking related content",
					Validator: func(r TestResult) ValidationResult {
						lower := strings.ToLower(r.Response)
						if strings.Contains(lower, "cast") || strings.Contains(lower, "chunk") ||
							strings.Contains(lower, "abstract syntax tree") || strings.Contains(lower, "ast") {
							return ValidationResult{Passed: true, Message: "PDF content successfully extracted"}
						}
						return ValidationResult{Passed: false, Message: "PDF content not properly extracted"}
					},
				},
				{
					Type:        "performance",
					Description: "Response should be under 30K characters (token limit)",
					Validator: func(r TestResult) ValidationResult {
						if len(r.Response) <= 30000 {
							return ValidationResult{Passed: true, Message: fmt.Sprintf("Response size OK: %d chars", len(r.Response))}
						}
						return ValidationResult{Passed: false, Message: fmt.Sprintf("Response too large: %d chars (limit: 30K)", len(r.Response))}
					},
				},
			},
		},
		{
			Name:             "Research Query with PDF Source Citation",
			Query:            "cAST chunking",
			ExpectedBehavior: "Should find and cite ArXiv PDF in sources",
			ValidationRules: []ValidationRule{
				{
					Type:        "content",
					Description: "Should contain cAST chunking information",
					Validator: func(r TestResult) ValidationResult {
						lower := strings.ToLower(r.Response)
						if strings.Contains(lower, "cast") && strings.Contains(lower, "chunk") {
							return ValidationResult{Passed: true, Message: "cAST chunking content found"}
						}
						return ValidationResult{Passed: false, Message: "cAST chunking content not found"}
					},
				},
				{
					Type:        "citation",
					Description: "Should cite ArXiv PDF in sources",
					Validator: func(r TestResult) ValidationResult {
						if strings.Contains(r.Response, "https://arxiv.org/pdf/2506.15655") ||
							strings.Contains(r.Response, "https://arxiv.org/abs/2506.15655") {
							return ValidationResult{Passed: true, Message: "ArXiv PDF cited in sources"}
						}
						return ValidationResult{Passed: false, Message: "ArXiv PDF not found in sources"}
					},
				},
				{
					Type:        "citation",
					Description: "Citations should be sequentially numbered",
					Validator:   validateCitations,
				},
			},
		},
	}
}

func runSequentialTests(env Environment, testCases []TestCase) []TestResult {
	var results []TestResult

	for i, tc := range testCases {
		if tc.Concurrent {
			continue // Skip concurrent tests in this phase
		}

		log.Printf("Test %d/%d: %s", i+1, len(testCases), tc.Name)

		result := executeTest(env, tc)
		results = append(results, result)

		// Check for critical failure
		if tc.Critical && result.Error != nil {
			log.Printf("CRITICAL ERROR: %v", result.Error)
			log.Println("Failing fast due to critical error")
			break
		}

		// Validate result
		var validationErrors []string
		testPassed := true
		for _, rule := range tc.ValidationRules {
			vr := rule.Validator(result)
			if !vr.Passed {
				validationErrors = append(validationErrors, fmt.Sprintf("[%s] %s", rule.Type, vr.Message))
				testPassed = false
			}
		}

		if len(validationErrors) > 0 {
			log.Printf("  ⚠️  Validation issues: %s", strings.Join(validationErrors, "; "))
		} else {
			log.Printf("  ✓ All validations passed")
		}

		log.Printf("  Duration: %v", result.Duration)

		// Print failed test details for debugging (but not for negative tests that pass validation)
		if !testPassed && (!tc.ExpectError || len(validationErrors) != 0) {
			log.Println("  === Failed Test Details ===")
			if result.Error != nil {
				log.Printf("  Error: %v", result.Error)
			}
			if result.Response != "" {
				responsePreview := result.Response
				if len(responsePreview) > 500 {
					responsePreview = responsePreview[:500] + "..."
				}
				log.Printf("  Response Preview:\n  %s", strings.ReplaceAll(responsePreview, "\n", "\n  "))
			} else {
				log.Println("  Response: <empty>")
			}
			log.Println("  === End Failed Test Details ===")
		}

		// Brief pause between tests to avoid rate limiting
		if i < len(testCases)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return results
}

func runConcurrentTests(env Environment) ConcurrentTestResult {
	log.Printf("Executing %d concurrent queries...", CONCURRENT_QUERIES)

	var wg sync.WaitGroup
	results := make([]TestResult, CONCURRENT_QUERIES)
	startTime := time.Now()

	for i := 0; i < CONCURRENT_QUERIES; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			tc := TestCase{
				Name:  fmt.Sprintf("Concurrent Query %d", index+1),
				Query: fmt.Sprintf("Best practices for microservices architecture pattern %d", index+1),
				ValidationRules: []ValidationRule{
					{
						Type:        "content",
						Description: "Should return microservices content",
						Validator: func(r TestResult) ValidationResult {
							if strings.Contains(strings.ToLower(r.Response), "microservice") {
								return ValidationResult{Passed: true}
							}
							return ValidationResult{Passed: false, Message: "No microservices content"}
						},
					},
				},
			}

			results[index] = executeTest(env, tc)
		}(i)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	// Analyze concurrent results
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Error == nil {
			successCount++
		} else {
			errorCount++
		}
	}

	return ConcurrentTestResult{
		TotalTime:    totalTime,
		ErrorCount:   errorCount,
		SuccessCount: successCount,
		SuccessRate:  float64(successCount) / float64(CONCURRENT_QUERIES) * 100,
	}
}

func executeTest(env Environment, tc TestCase) TestResult {
	startTime := time.Now()

	var req MCPRequest

	if tc.IsFetchURL {
		// Execute fetch_url test
		args := map[string]interface{}{
			"url": tc.FetchURL,
		}

		req = MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: ToolCallParams{
				Name:      "fetch_url",
				Arguments: args,
			},
		}
	} else {
		// Execute research_iteratively test
		args := map[string]interface{}{
			"query": tc.Query,
		}
		if tc.PreviousQuery != nil {
			args["previous_query"] = *tc.PreviousQuery
		}

		req = MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: ToolCallParams{
				Name:      "research_iteratively",
				Arguments: args,
			},
		}
	}

	resp, err := callMCP(env, "/mcp", req)
	endTime := time.Now()

	result := TestResult{
		TestCase:  tc,
		StartTime: startTime,
		EndTime:   endTime,
		Duration:  endTime.Sub(startTime),
		Error:     err,
	}

	// Handle network/HTTP errors
	if err != nil {
		return result
	}

	// Handle MCP-level errors vs successes
	if resp.Error != nil {
		// MCP returned an error response
		if tc.ExpectError {
			// This test expects an error - check if it's the right type
			if resp.Error.Code == tc.ExpectedErrorCode {
				// SUCCESS: Got the expected error code
				result.Error = fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
				// Note: We still set Error, but analyzeResults will treat this as success for ExpectError tests
			} else {
				// FAILURE: Got an error, but wrong error code
				result.Error = fmt.Errorf("expected MCP error %d, got error %d: %s", tc.ExpectedErrorCode, resp.Error.Code, resp.Error.Message)
			}
		} else {
			// This test doesn't expect an error - this is a failure
			result.Error = fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
	} else if resp.Result != nil {
		// MCP returned a success response
		if tc.ExpectError {
			// This test expected an error but got success - this is a failure
			result.Error = fmt.Errorf("expected error (code %d) but got successful response", tc.ExpectedErrorCode)
		} else {
			// Normal success case - parse the MCP-compliant response
			var toolResult MCPToolResult
			if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
				result.Error = fmt.Errorf("failed to parse MCP tool result: %v", err)
			} else if len(toolResult.Content) == 0 {
				result.Error = fmt.Errorf("MCP tool result has no content blocks")
			} else {
				// Extract text from first content block
				result.Response = toolResult.Content[0].Text
			}
		}
	}

	return result
}

func callMCP(env Environment, path string, req MCPRequest) (*MCPResponse, error) {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	httpReq, err := http.NewRequest("POST", env.BaseURL+path, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: REQUEST_TIMEOUT,
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(body, &mcpResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// CHANGED: Return the response even if it contains an MCP error
	// Let the caller decide how to handle MCP errors based on test expectations
	return &mcpResp, nil
}

func validateCitations(result TestResult) ValidationResult {
	if result.Error != nil {
		return ValidationResult{Passed: false, Message: "Cannot validate citations due to error"}
	}

	// Check that response has citations and a Sources Used section
	if !strings.Contains(result.Response, "[") || !strings.Contains(result.Response, "Sources Used:") {
		return ValidationResult{
			Passed:  false,
			Message: "No citations or Sources Used section found",
		}
	}

	// Extract the Sources Used section
	sourcesIndex := strings.Index(result.Response, "Sources Used:")
	if sourcesIndex == -1 {
		return ValidationResult{
			Passed:  false,
			Message: "Sources Used section not found",
		}
	}

	sourcesSection := result.Response[sourcesIndex:]

	// Check that sources are listed sequentially in the Sources Used section
	foundSequential := true
	lastFound := 0

	for i := 1; i <= 20; i++ {
		sourcePattern := fmt.Sprintf("[%d]", i)
		if strings.Contains(sourcesSection, sourcePattern) {
			if lastFound > 0 && i != lastFound+1 {
				foundSequential = false
				break
			}
			lastFound = i
		}
	}

	if !foundSequential {
		return ValidationResult{
			Passed:  false,
			Message: "Sources in 'Sources Used' section are not numbered sequentially",
		}
	}

	if lastFound > 0 {
		return ValidationResult{
			Passed:  true,
			Message: fmt.Sprintf("Sources properly listed sequentially from [1] to [%d]", lastFound),
		}
	}

	return ValidationResult{
		Passed:  false,
		Message: "No numbered sources found in Sources Used section",
	}
}

func analyzeResults(report *TestReport, results []TestResult) {
	report.TotalTests = len(results)

	var durations []time.Duration
	for _, r := range results {
		durations = append(durations, r.Duration)

		// Check if test passed - special handling for expected error tests
		var passed bool
		if r.TestCase.ExpectError {
			// For negative tests, success means we got the expected error
			passed = r.Error != nil && !strings.Contains(r.Error.Error(), "Expected MCP error")
		} else {
			// For positive tests, success means no error
			passed = r.Error == nil
		}

		// Run validation rules
		for _, rule := range r.TestCase.ValidationRules {
			vr := rule.Validator(r)
			if !vr.Passed {
				passed = false
				if r.Error == nil || (r.TestCase.ExpectError && r.Error != nil) {
					// Create a failed test entry for validation failures
					failed := FailedTest{
						Name:  r.TestCase.Name,
						Error: "Validation failures",
					}
					failed.ValidationErrors = append(failed.ValidationErrors, vr.Message)
					report.FailedTests = append(report.FailedTests, failed)
				}
			}
		}

		// Add to failed tests only if the test actually failed
		if !passed {
			// Avoid duplicate entries if we already added validation failures
			alreadyAdded := false
			for _, ft := range report.FailedTests {
				if ft.Name == r.TestCase.Name {
					alreadyAdded = true
					break
				}
			}

			if !alreadyAdded && r.Error != nil {
				report.FailedTests = append(report.FailedTests, FailedTest{
					Name:  r.TestCase.Name,
					Error: r.Error.Error(),
				})
			}
		}

		if passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	// Calculate metrics
	if len(results) > 0 {
		report.SuccessRate = float64(report.Passed) / float64(report.TotalTests) * 100

		totalDuration := time.Duration(0)
		for _, d := range durations {
			totalDuration += d
		}
		report.AverageDuration = totalDuration / time.Duration(len(results))

		// Calculate performance percentiles
		report.PerformanceMetrics = calculatePerformanceMetrics(durations)
	}
}

func calculatePerformanceMetrics(durations []time.Duration) PerformanceMetrics {
	if len(durations) == 0 {
		return PerformanceMetrics{}
	}

	// Simple min/max for now
	metrics := PerformanceMetrics{
		FastestQuery: durations[0],
		SlowestQuery: durations[0],
	}

	for _, d := range durations {
		if d < metrics.FastestQuery {
			metrics.FastestQuery = d
		}
		if d > metrics.SlowestQuery {
			metrics.SlowestQuery = d
		}
	}

	// Simple percentile calculations (would need sorting for accurate percentiles)
	metrics.P50 = durations[len(durations)/2]
	metrics.P90 = durations[len(durations)*9/10]
	metrics.P99 = durations[len(durations)*99/100]

	return metrics
}

func displayReport(report TestReport) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("ArguSeek QA Test Report - %s Environment\n", cases.Title(language.English).String(report.Environment))
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("Test Suite Version: %s\n", report.Version)
	fmt.Printf("Execution Time: %s\n", report.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Duration: %v\n", report.TotalDuration)
	fmt.Println()

	fmt.Println("=== Test Summary ===")
	fmt.Printf("Total Tests: %d\n", report.TotalTests)
	fmt.Printf("Passed: %d\n", report.Passed)
	fmt.Printf("Failed: %d\n", report.Failed)
	fmt.Printf("Success Rate: %.2f%%\n", report.SuccessRate)
	fmt.Printf("Average Duration: %v\n", report.AverageDuration)
	fmt.Println()

	if report.ConcurrentTestResult != nil {
		fmt.Println("=== Concurrent Test Results ===")
		fmt.Printf("Total Concurrent Queries: %d\n", CONCURRENT_QUERIES)
		fmt.Printf("Successful: %d\n", report.ConcurrentTestResult.SuccessCount)
		fmt.Printf("Failed: %d\n", report.ConcurrentTestResult.ErrorCount)
		fmt.Printf("Success Rate: %.2f%%\n", report.ConcurrentTestResult.SuccessRate)
		fmt.Printf("Total Time: %v\n", report.ConcurrentTestResult.TotalTime)
		fmt.Println()
	}

	fmt.Println("=== Performance Metrics ===")
	fmt.Printf("Fastest Query: %v\n", report.PerformanceMetrics.FastestQuery)
	fmt.Printf("Slowest Query: %v\n", report.PerformanceMetrics.SlowestQuery)
	if report.PerformanceMetrics.P50 > 0 {
		fmt.Printf("P50 (Median): %v\n", report.PerformanceMetrics.P50)
		fmt.Printf("P90: %v\n", report.PerformanceMetrics.P90)
		fmt.Printf("P99: %v\n", report.PerformanceMetrics.P99)
	}
	fmt.Println()

	if len(report.FailedTests) > 0 {
		fmt.Println("=== Failed Tests ===")
		for _, ft := range report.FailedTests {
			fmt.Printf("❌ %s\n", ft.Name)
			fmt.Printf("   Error: %s\n", ft.Error)
			for _, ve := range ft.ValidationErrors {
				fmt.Printf("   Validation: %s\n", ve)
			}
		}
		fmt.Println()
	}

	// Overall assessment
	fmt.Println("=== Assessment ===")
	if report.SuccessRate >= 95 {
		fmt.Println("✅ EXCELLENT: Test suite passed with high success rate")
	} else if report.SuccessRate >= 80 {
		fmt.Println("⚠️  GOOD: Test suite passed but some issues need attention")
	} else if report.SuccessRate >= 60 {
		fmt.Println("⚠️  CONCERNING: Multiple test failures detected")
	} else {
		fmt.Println("❌ CRITICAL: Major test failures - immediate attention required")
	}
}

func saveReport(env Environment, report TestReport) {
	filename := fmt.Sprintf("qa-report-%s-%d.json", report.Environment, time.Now().Unix())

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal report: %v", err)
		return
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("Failed to save report: %v", err)
		return
	}

	log.Printf("\nDetailed report saved to: %s", filename)
}

func stringPtr(s string) *string {
	return &s
}

// findProjectRoot walks up the directory tree to find the project root (directory containing go.mod)
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("could not get working directory: %v", err)
	}

	// Walk up the directory tree looking for go.mod
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return "", fmt.Errorf("could not find project root (no go.mod found)")
		}
		dir = parent
	}
}

// validateAPIKeys checks that required API keys are present in the environment
func validateAPIKeys() error {
	googleAPIKey := os.Getenv("GOOGLE_API_KEY")
	if googleAPIKey == "" {
		return fmt.Errorf("GOOGLE_API_KEY environment variable is required")
	}

	googleCSEID := os.Getenv("GOOGLE_CSE_ID")
	if googleCSEID == "" {
		return fmt.Errorf("GOOGLE_CSE_ID environment variable is required")
	}

	// GEMINI_API_KEY is optional - server will use GOOGLE_API_KEY as fallback
	log.Println("✓ Required API keys are present")
	return nil
}

// findAvailablePort finds an available port by letting the OS assign one
func findAvailablePort() (int, error) {
	// Let OS assign any available port (most robust, avoids race conditions)
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// startLocalServer starts the ArguSeek server as a subprocess
func startLocalServer() (int, *exec.Cmd, error) {
	log.Println("Starting local ArguSeek server...")

	// Validate API keys first
	if err := validateAPIKeys(); err != nil {
		return 0, nil, err
	}

	// Find available port
	port, err := findAvailablePort()
	if err != nil {
		return 0, nil, err
	}

	// Find project root for portable path resolution
	projectRoot, err := findProjectRoot()
	if err != nil {
		return 0, nil, err
	}

	// Build absolute path to server
	serverPath := filepath.Join(projectRoot, "cmd", "server", "main.go")

	// Start server process
	cmd := exec.Command("go", "run", serverPath, "-http")
	cmd.Dir = projectRoot // Run from project root so imports resolve
	cmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%d", port))

	// Capture stdout and stderr for debugging
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return 0, nil, fmt.Errorf("failed to start server: %v", err)
	}

	log.Printf("✓ Server process started (PID: %d, Port: %d)", cmd.Process.Pid, port)
	return port, cmd, nil
}

// waitForServerReady polls the health endpoint until server is ready
func waitForServerReady(port int) error {
	healthURL := fmt.Sprintf("http://localhost:%d/health", port)
	log.Printf("Waiting for server to be ready at %s...", healthURL)

	deadline := time.Now().Add(SERVER_STARTUP_TIMEOUT)
	for time.Now().Before(deadline) {
		resp, err := http.Get(healthURL)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			log.Println("✓ Server is ready")
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(HEALTH_CHECK_INTERVAL)
	}

	return fmt.Errorf("server did not become ready within %v", SERVER_STARTUP_TIMEOUT)
}

// stopLocalServer gracefully shuts down the server
func stopLocalServer(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	log.Printf("Shutting down server (PID: %d)...", cmd.Process.Pid)

	// Send SIGTERM for graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send SIGTERM: %v, attempting SIGKILL", err)
		return cmd.Process.Kill()
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-done:
		log.Println("✓ Server stopped gracefully")
		return nil
	case <-time.After(SERVER_SHUTDOWN_TIMEOUT):
		log.Println("Server did not stop gracefully, forcing shutdown...")
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill server: %v", err)
		}
		return nil
	}
}

// runContentProcessorValidation validates the new content processing pipeline
func runContentProcessorValidation() {
	log.Println("\n=== Content Processor Validation ===")
	log.Println("Testing new HTML to Markdown processing pipeline...")

	// Test cases specifically designed to validate content processing improvements
	testCases := []struct {
		name        string
		query       string
		validation  func(result string) bool
		description string
	}{
		{
			name:  "Code Block Preservation",
			query: "Python example to read a CSV file with pandas",
			validation: func(result string) bool {
				// Check for code blocks with proper formatting
				return strings.Contains(result, "```") || strings.Contains(result, "import pandas") || strings.Contains(result, "pd.read_csv")
			},
			description: "Validates that code blocks are preserved in markdown format",
		},
		{
			name:  "Documentation Structure",
			query: "React useEffect hook documentation and examples",
			validation: func(result string) bool {
				// Check for structured content with headers
				hasHeaders := strings.Contains(result, "##") || strings.Contains(result, "# ")
				hasCodeExample := strings.Contains(result, "useEffect") && (strings.Contains(result, "```") || strings.Contains(result, "() =>"))
				return hasHeaders || hasCodeExample
			},
			description: "Validates that documentation structure is preserved",
		},
		{
			name:  "Table Preservation",
			query: "HTTP status codes table with descriptions",
			validation: func(result string) bool {
				// Check for table-like content or structured lists
				hasStatusCodes := strings.Contains(result, "200") && strings.Contains(result, "404") && strings.Contains(result, "500")
				hasStructure := strings.Contains(result, "|") || (strings.Contains(result, "-") && hasStatusCodes)
				return hasStatusCodes && hasStructure
			},
			description: "Validates that table data is preserved in a structured format",
		},
		{
			name:  "Technical Content Extraction",
			query: "Kubernetes deployment YAML configuration example",
			validation: func(result string) bool {
				// Check for YAML structure preservation
				hasYAML := strings.Contains(result, "apiVersion") || strings.Contains(result, "kind:") || strings.Contains(result, "metadata:")
				hasCodeBlock := strings.Contains(result, "```") || strings.Contains(result, "yaml")
				return hasYAML && (hasCodeBlock || strings.Contains(result, "deployment"))
			},
			description: "Validates technical content and configuration preservation",
		},
		{
			name:  "Main Content Focus",
			query: "What is machine learning bias and how to prevent it",
			validation: func(result string) bool {
				// Check that main content is extracted without navigation/ads
				hasRelevantContent := strings.Contains(strings.ToLower(result), "bias") && strings.Contains(strings.ToLower(result), "machine learning")
				noNavigationText := !strings.Contains(strings.ToLower(result), "click here") && !strings.Contains(strings.ToLower(result), "subscribe")
				return hasRelevantContent && noNavigationText
			},
			description: "Validates main content extraction without noise",
		},
	}

	// Use dev environment for testing
	env := getEnvironment("dev")
	client := &http.Client{Timeout: REQUEST_TIMEOUT}

	successCount := 0
	for _, tc := range testCases {
		log.Printf("\n📋 Test Case: %s", tc.name)
		log.Printf("   Query: %s", tc.query)
		log.Printf("   Validation: %s", tc.description)

		// Execute research query
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      1,
			Method:  "tools/call",
			Params: ToolCallParams{
				Name: "research_iteratively",
				Arguments: map[string]interface{}{
					"query": tc.query,
				},
			},
		}

		body, _ := json.Marshal(req)
		httpReq, _ := http.NewRequest("POST", env.BaseURL+"/mcp", bytes.NewBuffer(body))
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(httpReq)
		if err != nil {
			log.Printf("   ❌ Request failed: %v", err)
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		respBody, _ := io.ReadAll(resp.Body)
		var mpcResp MCPResponse
		if err := json.Unmarshal(respBody, &mpcResp); err != nil {
			log.Printf("   ❌ Failed to parse MCP response: %v", err)
			continue
		}

		if mpcResp.Error != nil {
			log.Printf("   ❌ MCP Error: %s", mpcResp.Error.Message)
			continue
		}

		// Extract result from MCP-compliant format
		var toolResult MCPToolResult
		if err := json.Unmarshal(mpcResp.Result, &toolResult); err != nil {
			log.Printf("   ❌ Failed to parse MCP tool result: %v", err)
			continue
		}
		if len(toolResult.Content) == 0 {
			log.Printf("   ❌ MCP tool result has no content blocks")
			continue
		}
		result := toolResult.Content[0].Text

		// Run validation
		if tc.validation(result) {
			log.Printf("   ✅ PASSED - Content processing validated")
			successCount++

			// Show sample of processed content
			sample := result
			if len(sample) > 200 {
				sample = sample[:200] + "..."
			}
			log.Printf("   Sample: %s", strings.ReplaceAll(sample, "\n", " "))
		} else {
			log.Printf("   ❌ FAILED - Expected content features not found")
			log.Printf("   Response length: %d characters", len(result))
		}
	}

	// Summary
	log.Printf("\n=== Content Processor Validation Summary ===")
	log.Printf("Total tests: %d", len(testCases))
	log.Printf("Passed: %d", successCount)
	log.Printf("Failed: %d", len(testCases)-successCount)
	log.Printf("Success rate: %.1f%%", float64(successCount)/float64(len(testCases))*100)

	if successCount == len(testCases) {
		log.Println("\n✅ All content processing tests PASSED!")
		log.Println("The new HTML to Markdown pipeline is working correctly.")
	} else {
		log.Println("\n⚠️  Some content processing tests failed.")
		log.Println("Review the failed tests to identify issues with the new pipeline.")
	}
}
