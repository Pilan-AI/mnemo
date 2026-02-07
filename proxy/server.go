// Package proxy provides an HTTP proxy that sits between AI coding tools and
// the Claude API. It intercepts requests to inject relevant project context
// from mnemo's search index into the system prompt, and tracks token usage
// from responses.
//
// The proxy listens on a local port and forwards to the upstream Claude API,
// adding an "X-Mnemo-Tracked" header to indicate context was injected.
package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// Server is the mnemo HTTP proxy that intercepts Claude API requests.
type Server struct {
	port      string
	upstream  string
	server    *http.Server
	mu        sync.RWMutex
	sessionID string
}

type ClaudeRequest struct {
	Model       string          `json:"model"`
	Messages    []ClaudeMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	System      string          `json:"system,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type ClaudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID      string         `json:"id"`
	Type    string         `json:"type"`
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
	Model   string         `json:"model"`
	Usage   UsageInfo      `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type UsageInfo struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// NewServer creates a proxy server that listens on port and forwards to upstream.
func NewServer(port, upstream string) *Server {
	return &Server{
		port:      port,
		upstream:  upstream,
		server:    &http.Server{},
		mu:        sync.RWMutex{},
		sessionID: fmt.Sprintf("proxy-%d", time.Now().Unix()),
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", s.handleMessages)
	mux.HandleFunc("/v1/messages/", s.handleMessages)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"status":"ok","mnemo":"active"}`)
	})
	mux.HandleFunc("/stats", s.handleStats)

	s.server.Handler = mux
	s.server.Addr = s.port

	log.Printf("Mnemo proxy listening on %s, forwarding to %s\n", s.port, s.upstream)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.Body.Close() }()

	var req ClaudeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("Failed to unmarshal request: %v\n", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	project, query := s.extractContext(req.Messages)
	mnemoContext := s.getMnemoContext(project, query)

	if mnemoContext != "" {
		req.System = strings.TrimSpace(req.System) + "\n\n---\n## Project Memory (mnemo)\n" + mnemoContext + "\n---"
	}

	upstreamResp, rawBody, err := s.forwardToUpstream(&req)
	if err != nil {
		log.Printf("Failed to forward to upstream: %v\n", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}

	s.trackTokenUsage(upstreamResp, req.Model)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Mnemo-Tracked", "true")
	_, _ = w.Write(rawBody)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := db.GetTokenStats(30)
	if err != nil {
		http.Error(w, "Failed to get stats", http.StatusInternalServerError)
		return
	}

	byProvider, _ := db.GetTokenStatsByProvider(30)

	response := map[string]interface{}{
		"total":       stats,
		"by_provider": byProvider,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) trackTokenUsage(resp *ClaudeResponse, model string) {
	if resp == nil {
		return
	}

	usage := db.TokenUsage{
		SessionID:    s.sessionID,
		Model:        model,
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		CostUSD:      calculateCost(model, resp.Usage.InputTokens, resp.Usage.OutputTokens),
		Provider:     "anthropic",
		Timestamp:    time.Now(),
	}

	if err := db.InsertTokenUsage(usage); err != nil {
		log.Printf("Failed to track token usage: %v\n", err)
	}
}

// calculateCost estimates USD cost based on Anthropic's per-token pricing.
// Rates are hardcoded for Claude 3.x models (Opus/Sonnet/Haiku).
func calculateCost(model string, inputTokens, outputTokens int) float64 {
	var inputRate, outputRate float64

	switch {
	case strings.Contains(model, "opus"):
		inputRate = 15.0 / 1_000_000
		outputRate = 75.0 / 1_000_000
	case strings.Contains(model, "sonnet"):
		inputRate = 3.0 / 1_000_000
		outputRate = 15.0 / 1_000_000
	case strings.Contains(model, "haiku"):
		inputRate = 0.25 / 1_000_000
		outputRate = 1.25 / 1_000_000
	default:
		inputRate = 3.0 / 1_000_000
		outputRate = 15.0 / 1_000_000
	}

	return float64(inputTokens)*inputRate + float64(outputTokens)*outputRate
}

func (s *Server) extractContext(messages []ClaudeMessage) (project, query string) {
	if len(messages) == 0 {
		return "", ""
	}

	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			content := messages[i].Content
			project = s.extractProject(content)
			query = content
			break
		}
	}

	return project, query
}

func (s *Server) extractProject(content string) string {
	lower := strings.ToLower(content)
	keywords := []string{
		"project", "working on", "in ", "directory ",
		"repository", "repo ", "folder ",
	}

	for _, kw := range keywords {
		if idx := strings.Index(lower, kw); idx != -1 {
			after := content[idx+len(kw):]
			words := strings.Fields(after)
			if len(words) > 0 {
				return strings.ToTitle(words[0])
			}
		}
	}

	if pwd, err := os.Getwd(); err == nil {
		return extractProjectFromPath(pwd)
	}

	return "current-project"
}

func extractProjectFromPath(path string) string {
	parts := strings.Split(path, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

// getMnemoContext searches mnemo for relevant past sessions and returns a
// formatted summary to inject into the system prompt. Uses the project name
// plus the first couple words of the user's query as the search key.
func (s *Server) getMnemoContext(project, query string) string {
	if project == "" && query == "" {
		return ""
	}

	searchQuery := project
	if query != "" {
		words := strings.Fields(query)
		if len(words) > 0 {
			searchQuery = project + " " + words[0]
			if len(words) > 1 {
				searchQuery += " " + words[1]
			}
		}
	}

	results, err := db.Search(searchQuery, 5)
	if err != nil {
		log.Printf("Error searching mnemo: %v\n", err)
		return ""
	}

	if len(results) == 0 {
		return ""
	}

	var ctx strings.Builder
	ctx.WriteString("Based on your previous work:\n\n")

	for i, r := range results {
		ctx.WriteString(fmt.Sprintf("%d. %s\n", i+1, r.Snippet))
		if r.Role != "" {
			ctx.WriteString(fmt.Sprintf("   (%s)\n", r.Role))
		}
	}

	return ctx.String()
}

func (s *Server) forwardToUpstream(req *ClaudeRequest) (*ClaudeResponse, []byte, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, nil, err
	}

	upstreamReq, err := http.NewRequest("POST", s.upstream+"/v1/messages", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, nil, err
	}

	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	upstreamReq.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	var claudeResp ClaudeResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		return nil, body, nil
	}

	return &claudeResp, body, nil
}
