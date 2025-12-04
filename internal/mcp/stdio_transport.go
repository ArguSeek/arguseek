package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// ServeStdio runs the MCP handler in stdio mode, reading JSON-RPC requests from stdin
// and writing responses to stdout. This is the standard MCP transport for local integrations.
//
// The function blocks until stdin is closed (EOF) or a termination signal is received.
// All log output is redirected to stderr to keep stdout clean for MCP messages.
func (h *Handler) ServeStdio(ctx context.Context) error {
	// Ensure logs go to stderr in stdio mode
	log.SetOutput(os.Stderr)
	log.Println("Starting MCP server in stdio mode")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create cancellable context for request processing
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating shutdown", sig)
		cancel()
	}()

	return h.serveStdioWithIO(ctx, os.Stdin, os.Stdout)
}

// serveStdioWithIO is the testable implementation that accepts injectable I/O streams.
// This allows tests to use in-memory readers/writers instead of os.Stdin/os.Stdout.
func (h *Handler) serveStdioWithIO(ctx context.Context, reader io.Reader, writer io.Writer) error {
	scanner := bufio.NewScanner(reader)
	bufferedWriter := bufio.NewWriter(writer)
	defer func() { _ = bufferedWriter.Flush() }()

	// Read requests line by line until EOF or shutdown
	for scanner.Scan() {
		// Check if context was cancelled (shutdown signal)
		select {
		case <-ctx.Done():
			log.Println("Shutdown signal received, stopping request processing")
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue // Skip empty lines
		}

		// Parse JSON-RPC request
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			// Send parse error response
			errorResp := Response{
				JSONRPC: "2.0",
				ID:      nil, // No ID available since parse failed
				Error: &Error{
					Code:    -32700,
					Message: fmt.Sprintf("Parse error: %v", err),
				},
			}
			if err := writeStdioResponse(bufferedWriter, errorResp); err != nil {
				log.Printf("Failed to write error response: %v", err)
			}
			continue
		}

		// Process request using transport-agnostic logic
		response, err := h.ProcessRequest(ctx, req)
		if err != nil {
			log.Printf("Internal error processing request: %v", err)
			errorResp := Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32603,
					Message: fmt.Sprintf("Internal error: %v", err),
				},
			}
			if err := writeStdioResponse(bufferedWriter, errorResp); err != nil {
				log.Printf("Failed to write error response: %v", err)
			}
			continue
		}

		// Notifications return nil - no response per JSON-RPC 2.0 spec
		if response == nil {
			continue
		}

		// Write response to stdout
		if err := writeStdioResponse(bufferedWriter, *response); err != nil {
			log.Printf("Failed to write response: %v", err)
			return err
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		if err == io.EOF {
			log.Println("Stdin closed (EOF), shutting down")
			return nil
		}
		log.Printf("Error reading from stdin: %v", err)
		return err
	}

	log.Println("MCP server stopped")
	return nil
}

// writeStdioResponse writes a JSON-RPC response to stdout as a single line.
// Per MCP spec: messages are newline-delimited and must not contain embedded newlines.
func writeStdioResponse(writer *bufio.Writer, response Response) error {
	// Marshal response to JSON (compact, no newlines)
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	// Write JSON followed by newline delimiter
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush immediately to ensure message is sent
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}
