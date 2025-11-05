package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"

	"arguseek/internal/agent"
	"arguseek/internal/request"
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

	if req.JSONRPC != "2.0" {
		writeError(w, req.ID, -32600, "Invalid Request", http.StatusBadRequest)
		return
	}

	switch req.Method {
	case "initialize":
		h.handleInitialize(w, req)
	case "tools/list":
		h.handleToolsList(w, req)
	case "tools/call":
		h.handleToolsCall(w, r, req)
	default:
		writeError(w, req.ID, -32601, "Method not found", http.StatusNotFound)
	}
}

func (h *Handler) handleInitialize(w http.ResponseWriter, req Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: Tools{
				ListChanged: false,
			},
		},
		ServerInfo: ServerInfo{
			Name:    "arguseek",
			Version: "0.3.3",
		},
	}

	response := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) handleToolsList(w http.ResponseWriter, req Request) {
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

	response := Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) handleToolsCall(w http.ResponseWriter, r *http.Request, req Request) {
	// Parse the params as a tools/call request
	paramsJSON, err := json.Marshal(req.Params)
	if err != nil {
		writeError(w, req.ID, -32602, "Invalid params", http.StatusBadRequest)
		return
	}

	var params Params
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		writeError(w, req.ID, -32602, "Invalid params", http.StatusBadRequest)
		return
	}

	switch params.Name {
	case "research_iteratively":
		// Validate query length (prevents ReDoS and resource exhaustion)
		if err := validateQueryLength(params.Arguments.Query); err != nil {
			writeError(w, req.ID, -32602, fmt.Sprintf("Invalid query: %v", err), http.StatusBadRequest)
			return
		}
		if params.Arguments.PreviousQuery != nil && *params.Arguments.PreviousQuery != "" {
			if err := validateQueryLength(*params.Arguments.PreviousQuery); err != nil {
				writeError(w, req.ID, -32602, fmt.Sprintf("Invalid previous query: %v", err), http.StatusBadRequest)
				return
			}
		}

		result, err := h.agent.ResearchIteratively(r.Context(), params.Arguments.Query, params.Arguments.PreviousQuery)
		if err != nil {
			writeError(w, req.ID, -32603, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
			return
		}

		response := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	case "fetch_url":
		// Validate URL (prevents SSRF attacks)
		if err := validateURL(params.Arguments.URL); err != nil {
			writeError(w, req.ID, -32602, fmt.Sprintf("Invalid URL: %v", err), http.StatusBadRequest)
			return
		}

		result, err := h.agent.FetchURL(r.Context(), params.Arguments.URL, params.Arguments.LookingFor)
		if err != nil {
			writeError(w, req.ID, -32603, fmt.Sprintf("Internal error: %v", err), http.StatusInternalServerError)
			return
		}

		response := Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)

	default:
		writeError(w, req.ID, -32602, "Invalid params: unknown tool", http.StatusBadRequest)
		return
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	json.NewEncoder(w).Encode(response)
}
