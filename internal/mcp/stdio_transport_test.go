package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
)


// Helper function to create a valid initialize request
func createInitializeRequest(id int) string {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}
	data, _ := json.Marshal(req)
	return string(data)
}

// Helper function to create a valid tools/list request
func createToolsListRequest(id int) string {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "tools/list",
	}
	data, _ := json.Marshal(req)
	return string(data)
}

// Helper function to assert a line is valid JSON-RPC and return the parsed response
func assertValidJSONRPCLine(t *testing.T, line string) Response {
	t.Helper()
	line = strings.TrimSpace(line)

	var resp Response
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		t.Fatalf("Not valid JSON-RPC: %v\nGot: %s", err, line)
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected jsonrpc '2.0', got '%s'", resp.JSONRPC)
	}

	return resp
}

// Helper function to assert stdout has no log pollution
func assertNoLogPollution(t *testing.T, output string) {
	t.Helper()
	badPatterns := []string{
		"[DEBUG]",
		"[INFO]",
		"Starting MCP",
		"Shutdown signal",
		"MCP server stopped",
		"Received signal",
	}

	for _, pattern := range badPatterns {
		if strings.Contains(output, pattern) {
			t.Errorf("Stdout pollution detected: found log pattern %q in output:\n%s", pattern, output)
		}
	}
}

// TestServeStdio_ValidRequest_OutputsCleanJSON tests that valid requests produce only JSON-RPC output
func TestServeStdio_ValidRequest_OutputsCleanJSON(t *testing.T) {
	// Create handler with mock agent
	handler := NewHandler(nil) // Handler doesn't use agent for initialize/tools/list

	// Prepare input: valid initialize request
	input := createInitializeRequest(1) + "\n"
	stdin := strings.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	done := make(chan struct{})

	// Redirect log output to our stderr buffer for this test
	oldLogOutput := os.Stderr
	oldLogWriter := log.Writer()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe for stderr capture: %v", err)
	}
	os.Stderr = w
	log.SetOutput(w)
	defer func() {
		os.Stderr = oldLogOutput
		log.SetOutput(oldLogWriter)
	}()

	// Capture stderr in goroutine
	go func() {
		defer close(done)
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				stderr.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}()

	// Run server
	ctx := context.Background()
	err = handler.serveStdioWithIO(ctx, stdin, &stdout)
	_ = w.Close()
	<-done // Wait for stderr capture to complete

	if err != nil {
		t.Fatalf("serveStdioWithIO returned error: %v", err)
	}

	// Verify stdout contains only valid JSON-RPC
	stdoutStr := stdout.String()
	assertNoLogPollution(t, stdoutStr)

	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 response line, got %d", len(lines))
	}

	resp := assertValidJSONRPCLine(t, lines[0])
	// Handle both int and float64 from JSON unmarshaling
	var actualID int
	switch v := resp.ID.(type) {
	case int:
		actualID = v
	case float64:
		actualID = int(v)
	default:
		t.Fatalf("Unexpected ID type: %T", resp.ID)
	}
	if actualID != 1 {
		t.Errorf("Expected response ID 1, got %v", actualID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got: %+v", resp.Error)
	}

	// Verify logs were written to stderr (not stdout)
	_ = r.Close()
	stderrStr := stderr.String()
	if len(stderrStr) == 0 {
		t.Error("Expected logs in stderr, but stderr is empty")
	}
}

// TestServeStdio_ParseError_OutputsErrorJSON tests that parse errors produce -32700 error code
func TestServeStdio_ParseError_OutputsErrorJSON(t *testing.T) {
	handler := NewHandler(nil)

	// Prepare input: invalid JSON
	input := "not valid json\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	_ = handler.serveStdioWithIO(ctx, stdin, &stdout)

	// Verify stdout contains error response
	stdoutStr := stdout.String()
	assertNoLogPollution(t, stdoutStr)

	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 error response line, got %d", len(lines))
	}

	resp := assertValidJSONRPCLine(t, lines[0])
	if resp.Error == nil {
		t.Fatal("Expected error response, got nil")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("Expected error code -32700 (parse error), got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "Parse error") {
		t.Errorf("Expected parse error message, got: %s", resp.Error.Message)
	}
}

// TestServeStdio_MultipleRequests_NewlineDelimited tests that multiple requests are handled correctly
func TestServeStdio_MultipleRequests_NewlineDelimited(t *testing.T) {
	handler := NewHandler(nil)

	// Prepare input: 3 valid requests
	input := createInitializeRequest(1) + "\n" +
		createToolsListRequest(2) + "\n" +
		createToolsListRequest(3) + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	_ = handler.serveStdioWithIO(ctx, stdin, &stdout)

	// Verify stdout contains 3 response lines
	stdoutStr := stdout.String()
	assertNoLogPollution(t, stdoutStr)

	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 response lines, got %d", len(lines))
	}

	// Verify each line is valid JSON-RPC with correct ID
	for i, line := range lines {
		resp := assertValidJSONRPCLine(t, line)
		expectedID := i + 1
		// Handle both int and float64 from JSON unmarshaling
		var actualID int
		switch v := resp.ID.(type) {
		case int:
			actualID = v
		case float64:
			actualID = int(v)
		default:
			t.Fatalf("Unexpected ID type: %T", resp.ID)
		}
		if actualID != expectedID {
			t.Errorf("Line %d: expected ID %d, got %v", i, expectedID, resp.ID)
		}
	}
}

// TestServeStdio_EmptyLines_Skipped tests that empty lines are ignored
func TestServeStdio_EmptyLines_Skipped(t *testing.T) {
	handler := NewHandler(nil)

	// Prepare input with empty lines
	input := "\n\n" + createInitializeRequest(1) + "\n\n\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	_ = handler.serveStdioWithIO(ctx, stdin, &stdout)

	// Verify only 1 response (empty lines should be skipped)
	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 response (empty lines skipped), got %d", len(lines))
	}

	assertValidJSONRPCLine(t, lines[0])
}

// TestServeStdio_ContextCancellation_GracefulShutdown tests that context cancellation stops processing
func TestServeStdio_ContextCancellation_GracefulShutdown(t *testing.T) {
	handler := NewHandler(nil)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Prepare input: multiple requests, but we'll cancel before processing all
	input := createInitializeRequest(1) + "\n" +
		createToolsListRequest(2) + "\n" +
		createToolsListRequest(3) + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	// Cancel context immediately to test early termination
	cancel()

	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	// Should return context.Canceled error
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

// TestWriteStdioResponse_NoEmbeddedNewlines tests that responses have no embedded newlines
func TestWriteStdioResponse_NoEmbeddedNewlines(t *testing.T) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	// Create response with potentially problematic content
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]string{
			"content": "Line 1\nLine 2\nLine 3",
		},
	}

	err := writeStdioResponse(writer, resp)
	if err != nil {
		t.Fatalf("writeStdioResponse failed: %v", err)
	}

	output := buf.String()

	// Count newlines - should have exactly 1 (the delimiter)
	newlineCount := strings.Count(output, "\n")
	if newlineCount != 1 {
		t.Errorf("Expected exactly 1 newline (delimiter), got %d", newlineCount)
	}

	// Verify the response is valid JSON
	trimmed := strings.TrimSuffix(output, "\n")
	var parsed Response
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		t.Errorf("Response is not valid JSON: %v", err)
	}
}

// TestServeStdio_InvalidMethod_ReturnsMethodNotFound tests invalid method handling
func TestServeStdio_InvalidMethod_ReturnsMethodNotFound(t *testing.T) {
	handler := NewHandler(nil)

	// Create request with invalid method
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "invalid/method",
	}
	data, _ := json.Marshal(req)
	input := string(data) + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	_ = handler.serveStdioWithIO(ctx, stdin, &stdout)

	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")

	resp := assertValidJSONRPCLine(t, lines[0])
	if resp.Error == nil {
		t.Fatal("Expected error response for invalid method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("Expected error code -32601 (method not found), got %d", resp.Error.Code)
	}
}

// TestServeStdio_MalformedJSONRPC_ParseError tests various malformed JSON inputs
func TestServeStdio_MalformedJSONRPC_ParseError(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedCode int
	}{
		{"incomplete json", `{"jsonrpc": "2.0", "id": 1`, -32700},          // Parse error
		{"invalid characters", `{invalid}`, -32700},                        // Parse error
		{"empty object", `{}`, -32600},                                     // Invalid request (valid JSON but missing required fields)
		{"array instead of object", `[]`, -32700},                          // Parse error (wrong type)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := NewHandler(nil)
			stdin := strings.NewReader(tt.input + "\n")
			var stdout bytes.Buffer

			ctx := context.Background()
			_ = handler.serveStdioWithIO(ctx, stdin, &stdout)

			stdoutStr := stdout.String()
			if stdoutStr == "" {
				t.Fatal("Expected error response, got empty output")
			}

			lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
			resp := assertValidJSONRPCLine(t, lines[0])

			if resp.Error == nil {
				t.Fatal("Expected error response")
			}
			if resp.Error.Code != tt.expectedCode {
				t.Errorf("Expected error code %d, got %d", tt.expectedCode, resp.Error.Code)
			}
		})
	}
}

// TestWriteStdioResponse_ErrorFormats tests various error response formats
func TestWriteStdioResponse_ErrorFormats(t *testing.T) {
	tests := []struct {
		name     string
		response Response
		wantCode int
	}{
		{
			name: "parse error",
			response: Response{
				JSONRPC: "2.0",
				ID:      nil,
				Error: &Error{
					Code:    -32700,
					Message: "Parse error",
				},
			},
			wantCode: -32700,
		},
		{
			name: "method not found",
			response: Response{
				JSONRPC: "2.0",
				ID:      1,
				Error: &Error{
					Code:    -32601,
					Message: "Method not found",
				},
			},
			wantCode: -32601,
		},
		{
			name: "internal error",
			response: Response{
				JSONRPC: "2.0",
				ID:      1,
				Error: &Error{
					Code:    -32603,
					Message: "Internal error",
				},
			},
			wantCode: -32603,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := bufio.NewWriter(&buf)

			err := writeStdioResponse(writer, tt.response)
			if err != nil {
				t.Fatalf("writeStdioResponse failed: %v", err)
			}

			output := strings.TrimSpace(buf.String())
			var parsed Response
			if err := json.Unmarshal([]byte(output), &parsed); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if parsed.Error == nil {
				t.Fatal("Expected error in response")
			}
			if parsed.Error.Code != tt.wantCode {
				t.Errorf("Expected error code %d, got %d", tt.wantCode, parsed.Error.Code)
			}
		})
	}
}

// TestServeStdio_LargeRequest_HandledCorrectly tests handling of large JSON requests
func TestServeStdio_LargeRequest_HandledCorrectly(t *testing.T) {
	handler := NewHandler(nil)

	// Create a large request with long strings
	longString := strings.Repeat("a", 10000)
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    longString,
				"version": "1.0.0",
			},
		},
	}
	data, _ := json.Marshal(req)
	input := string(data) + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	if err != nil {
		t.Fatalf("Failed to handle large request: %v", err)
	}

	// Verify we got a valid response
	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 response, got %d", len(lines))
	}

	assertValidJSONRPCLine(t, lines[0])
}

// TestServeStdio_ConcurrentSafety tests that the handler is safe for concurrent requests
// Note: This is a basic smoke test. Real concurrent testing would require more sophisticated setup.
func TestServeStdio_ConcurrentSafety(t *testing.T) {
	handler := NewHandler(nil)

	// Create multiple requests
	input := createInitializeRequest(1) + "\n" +
		createToolsListRequest(2) + "\n" +
		createToolsListRequest(3) + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	if err != nil {
		t.Fatalf("Concurrent processing failed: %v", err)
	}

	// Verify all responses are present
	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 3 {
		t.Fatalf("Expected 3 responses, got %d", len(lines))
	}

	for i, line := range lines {
		resp := assertValidJSONRPCLine(t, line)
		if resp.Error != nil {
			t.Errorf("Response %d had error: %+v", i, resp.Error)
		}
	}
}

// TestServeStdio_EOFHandling tests graceful handling of EOF
func TestServeStdio_EOFHandling(t *testing.T) {
	handler := NewHandler(nil)

	// Create input that ends naturally (EOF after valid request)
	input := createInitializeRequest(1) + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	// EOF should not be treated as an error
	if err != nil {
		t.Errorf("Expected nil error on EOF, got: %v", err)
	}

	// Should have processed the request
	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 1 {
		t.Fatalf("Expected 1 response before EOF, got %d", len(lines))
	}

	assertValidJSONRPCLine(t, lines[0])
}

// Benchmark tests
func BenchmarkServeStdio_SingleRequest(b *testing.B) {
	handler := NewHandler(nil)
	input := createInitializeRequest(1) + "\n"

	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		var stdout bytes.Buffer
		ctx := context.Background()
		_ = handler.serveStdioWithIO(ctx, stdin, &stdout)
	}
}

func BenchmarkServeStdio_MultipleRequests(b *testing.B) {
	handler := NewHandler(nil)
	input := ""
	for i := 1; i <= 10; i++ {
		input += createToolsListRequest(i) + "\n"
	}

	for i := 0; i < b.N; i++ {
		stdin := strings.NewReader(input)
		var stdout bytes.Buffer
		ctx := context.Background()
		_ = handler.serveStdioWithIO(ctx, stdin, &stdout)
	}
}

func BenchmarkWriteStdioResponse(b *testing.B) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result: map[string]string{
			"status": "ok",
			"data":   "test response",
		},
	}

	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		_ = writeStdioResponse(writer, resp)
	}
}

// TestIsNotification tests the notification detection helper
func TestIsNotification(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"notifications/initialized", true},
		{"notifications/cancelled", true},
		{"notifications/progress", true},
		{"notifications/custom", true},
		{"initialize", false},
		{"tools/list", false},
		{"tools/call", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			got := isNotification(tt.method)
			if got != tt.want {
				t.Errorf("isNotification(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

// TestServeStdio_Notification_NoResponse tests that notifications don't produce responses
func TestServeStdio_Notification_NoResponse(t *testing.T) {
	handler := NewHandler(nil)

	// Create notification request (per MCP spec, notifications don't have IDs in practice,
	// but we test with the method pattern which is how we detect notifications)
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, _ := json.Marshal(req)
	input := string(data) + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	if err != nil {
		t.Fatalf("serveStdioWithIO returned error: %v", err)
	}

	// Verify stdout is empty - notifications should not produce responses
	stdoutStr := stdout.String()
	if stdoutStr != "" {
		t.Errorf("Expected no response for notification, got: %s", stdoutStr)
	}
}

// TestServeStdio_MixedRequestsAndNotifications tests handling of mixed traffic
func TestServeStdio_MixedRequestsAndNotifications(t *testing.T) {
	handler := NewHandler(nil)

	// Create mixed traffic: request, notification, request
	notification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	notificationData, _ := json.Marshal(notification)

	input := createInitializeRequest(1) + "\n" +
		string(notificationData) + "\n" +
		createToolsListRequest(2) + "\n"

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)

	if err != nil {
		t.Fatalf("serveStdioWithIO returned error: %v", err)
	}

	// Verify only 2 responses (notification should not produce response)
	stdoutStr := stdout.String()
	lines := strings.Split(strings.TrimSpace(stdoutStr), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 responses (notification skipped), got %d: %v", len(lines), lines)
	}

	// Verify response IDs are 1 and 2
	expectedIDs := []int{1, 2}
	for i, line := range lines {
		resp := assertValidJSONRPCLine(t, line)
		var actualID int
		switch v := resp.ID.(type) {
		case int:
			actualID = v
		case float64:
			actualID = int(v)
		default:
			t.Fatalf("Unexpected ID type: %T", resp.ID)
		}
		if actualID != expectedIDs[i] {
			t.Errorf("Response %d: expected ID %d, got %d", i, expectedIDs[i], actualID)
		}
	}
}

// TestProcessRequest_Notification_ReturnsNil tests that ProcessRequest returns nil for notifications
func TestProcessRequest_Notification_ReturnsNil(t *testing.T) {
	handler := NewHandler(nil)

	tests := []struct {
		method string
	}{
		{"notifications/initialized"},
		{"notifications/cancelled"},
		{"notifications/progress"},
		{"notifications/custom"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			req := Request{
				JSONRPC: "2.0",
				Method:  tt.method,
			}

			response, err := handler.ProcessRequest(context.Background(), req)

			if err != nil {
				t.Errorf("ProcessRequest returned error: %v", err)
			}
			if response != nil {
				t.Errorf("Expected nil response for notification %q, got: %+v", tt.method, response)
			}
		})
	}
}

// Example test showing expected usage pattern
func ExampleHandler_serveStdioWithIO() {
	handler := NewHandler(nil)

	// Simulate a client sending an initialize request
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}` + "\n"
	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	ctx := context.Background()
	err := handler.serveStdioWithIO(ctx, stdin, &stdout)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Output will be a valid JSON-RPC response
	fmt.Println("Response received and valid")
	// Output: Response received and valid
}
