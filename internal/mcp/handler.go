package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"arguseek/internal/agent"
	"arguseek/internal/request"
	"arguseek/internal/version"
)

type Handler struct {
	agent *agent.SearchAgent
}

func NewHandler(agent *agent.SearchAgent) *Handler {
	return &Handler{
		agent: agent,
	}
}

type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type Params struct {
	Name      string    `json:"name"`
	Arguments Arguments `json:"arguments"`
}

type Arguments struct {
	Query         string  `json:"query"`
	PreviousQuery *string `json:"previous_query,omitempty"`
	URL           string  `json:"url"`
	LookingFor    string  `json:"looking_for,omitempty"`
}

type InitializeParams struct {
	ProtocolVersion string      `json:"protocolVersion"`
	Capabilities    interface{} `json:"capabilities"`
	ClientInfo      ClientInfo  `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Capabilities struct {
	Tools Tools `json:"tools"`
}

type Tools struct {
	ListChanged bool `json:"listChanged"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties"`
	Required   []string               `json:"required"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type Response struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol content structures
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
}

// wrapInContent wraps a tool result (string or object) in MCP content array format
func wrapInContent(result interface{}) ToolCallResult {
	var text string

	// Convert result to string
	switch v := result.(type) {
	case string:
		text = v
	default:
		// For non-string results, marshal to JSON
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			log.Printf("Warning: Failed to JSON-marshal tool result, using string fallback: %v", err)
			text = fmt.Sprintf("%v", v)
		} else {
			text = string(jsonBytes)
		}
	}

	return ToolCallResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}

// isNotification returns true if the method is an MCP notification.
// Per JSON-RPC 2.0 spec: notifications MUST NOT receive responses.
// MCP notifications use the "notifications/" prefix by convention.
func isNotification(method string) bool {
	return strings.HasPrefix(method, "notifications/")
}

// ProcessRequest handles the core MCP protocol logic independent of transport.
// Returns a *Response (nil for notifications) and error for transport-agnostic handling.
// A nil response indicates the request was a notification and no response should be sent.
func (h *Handler) ProcessRequest(ctx context.Context, req Request) (*Response, error) {
	// Validate protocol version
	if req.JSONRPC != "2.0" {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32600,
				Message: "Invalid Request",
			},
		}, nil
	}

	// Handle notifications - no response per JSON-RPC 2.0 spec
	if isNotification(req.Method) {
		return nil, nil
	}

	switch req.Method {
	case "initialize":
		return h.processInitialize(req)
	case "tools/list":
		return h.processToolsList(req)
	case "tools/call":
		return h.processToolsCall(ctx, req)
	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32601,
				Message: "Method not found",
			},
		}, nil
	}
}

// HandleRequest processes MCP requests without authentication or input validation.
// This service is designed to be open-by-default for simplicity.
//
// For production deployments, add external security controls:
// - Authentication: Use reverse proxy, Cloud Run IAM, or API gateway
// - Rate limiting: Configure at infrastructure layer
// - Input validation: Add if needed for your deployment
//
// See PRODUCTION_SECURITY.md for detailed guidance.
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	// Use unified context - clean and obvious
	requestID := request.GetRequestID(r.Context())
	log.Printf("[%s] MCP request received: %s %s", requestID, r.Method, r.URL.Path)

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, nil, -32700, "Parse error", http.StatusBadRequest)
		return
	}

	// Process request using transport-agnostic logic
	response, err := h.ProcessRequest(r.Context(), req)
	if err != nil {
		writeError(w, req.ID, -32603, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
		return
	}

	// Notifications return nil - no response per JSON-RPC 2.0 spec
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Return appropriate HTTP status based on error presence
	statusCode := http.StatusOK
	if response.Error != nil {
		switch response.Error.Code {
		case -32700:
			statusCode = http.StatusBadRequest
		case -32600:
			statusCode = http.StatusBadRequest
		case -32601:
			statusCode = http.StatusNotFound
		case -32602:
			statusCode = http.StatusBadRequest
		case -32603:
			statusCode = http.StatusInternalServerError
		default:
			statusCode = http.StatusInternalServerError
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func (h *Handler) processInitialize(req Request) (*Response, error) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: Tools{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "arguseek",
			Version: version.Version,
		},
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, nil
}

func (h *Handler) processToolsList(req Request) (*Response, error) {
	tools := []Tool{
		{
			Name:        "research_iteratively",
			Description: "Perform iterative web research using ArguSeek's Google Search and Gemini integration",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The research query to investigate",
					},
					"previous_query": map[string]interface{}{
						"type":        "string",
						"description": "Previous query to build context and chain knowledge (optional)",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "fetch_url",
			Description: "Fetch and extract content from any webpage URL. Use this tool when you need to read, analyze, or extract information from a web page.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"url": map[string]interface{}{
						"type":        "string",
						"description": "The complete webpage URL to fetch content from. Must include protocol (http/https). Example: 'https://docs.example.com/api-guide'",
					},
					"looking_for": map[string]interface{}{
						"type":        "string",
						"description": "What information are you looking for? Describe naturally like: 'pricing plans', 'installation steps', 'error codes', 'API authentication'",
					},
				},
				Required: []string{"url"},
			},
		},
	}

	result := ToolsListResult{
		Tools: tools,
	}

	return &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}, nil
}

func (h *Handler) processToolsCall(ctx context.Context, req Request) (*Response, error) {
	// Parse the params as a tools/call request
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	var params Params
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params",
			},
		}, nil
	}

	switch params.Name {
	case "research_iteratively":
		// Validate query length (prevents ReDoS and resource exhaustion)
		if err := validateQueryLength(params.Arguments.Query); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32602,
					Message: fmt.Sprintf("Invalid query: %v", err),
				},
			}, nil
		}
		if params.Arguments.PreviousQuery != nil && *params.Arguments.PreviousQuery != "" {
			if err := validateQueryLength(*params.Arguments.PreviousQuery); err != nil {
				return &Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &Error{
						Code:    -32602,
						Message: fmt.Sprintf("Invalid previous query: %v", err),
					},
				}, nil
			}
		}

		result, err := h.agent.ResearchIteratively(ctx, params.Arguments.Query, params.Arguments.PreviousQuery)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32603,
					Message: fmt.Sprintf("Internal error: %v", err),
				},
			}, nil
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  wrapInContent(result),
		}, nil

	case "fetch_url":
		// Validate URL (prevents SSRF attacks)
		if err := validateURL(params.Arguments.URL); err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32602,
					Message: fmt.Sprintf("Invalid URL: %v", err),
				},
			}, nil
		}

		result, err := h.agent.FetchURL(ctx, params.Arguments.URL, params.Arguments.LookingFor)
		if err != nil {
			return &Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &Error{
					Code:    -32603,
					Message: fmt.Sprintf("Internal error: %v", err),
				},
			}, nil
		}

		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  wrapInContent(result),
		}, nil

	default:
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &Error{
				Code:    -32602,
				Message: "Invalid params: unknown tool",
			},
		}, nil
	}
}

// Input validation helpers (independent of authentication)
// These prevent SSRF, ReDoS, and resource exhaustion attacks

const maxQueryLength = 10000 // Per OWASP guidance for ReDoS prevention

// validateQueryLength checks query length to prevent ReDoS and memory exhaustion
func validateQueryLength(query string) error {
	if len(query) > maxQueryLength {
		return fmt.Errorf("query exceeds maximum length of %d characters", maxQueryLength)
	}
	return nil
}

// validateURL performs SSRF protection checks
func validateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Block file:// URIs
	if len(urlStr) >= 7 && urlStr[:7] == "file://" {
		return fmt.Errorf("file:// URLs are not allowed")
	}

	// Parse URL
	var err error
	var u *url.URL
	if u, err = url.Parse(urlStr); err != nil {
		return fmt.Errorf("invalid URL format: %v", err)
	}

	// Require http/https scheme
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https schemes are allowed")
	}

	// Extract hostname for IP validation
	hostname := u.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL must have a hostname")
	}

	// Block localhost variants
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" {
		return fmt.Errorf("localhost URLs are not allowed")
	}

	// Try to parse as IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		// Block private IP ranges
		if ip.IsPrivate() {
			return fmt.Errorf("private IP addresses are not allowed")
		}
		// Block link-local addresses (169.254.0.0/16, fe80::/10)
		if ip.IsLinkLocalUnicast() {
			return fmt.Errorf("link-local IP addresses are not allowed")
		}
		// Block loopback
		if ip.IsLoopback() {
			return fmt.Errorf("loopback IP addresses are not allowed")
		}
	} else {
		// It's a domain name - resolve and check all IPs
		ips, err := net.LookupIP(hostname)
		if err != nil {
			// Don't fail on DNS errors - let the fetcher handle it
			return nil
		}

		// Check all resolved IPs
		for _, ip := range ips {
			if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLoopback() {
				return fmt.Errorf("domain resolves to disallowed IP address")
			}
		}
	}

	return nil
}

func writeError(w http.ResponseWriter, id any, code int, message string, httpStatus int) {
	response := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode error response: %v", err)
	}
}
