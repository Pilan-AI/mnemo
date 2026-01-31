//go:build ignore

package proxy

import (
	"context"
	"database/sql"
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

type Server struct {
	port     string
	upstream string
	db       *sql.DB
	server   *http.Server
	mu       sync.RWMutex
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
	Content string `json:"content"`
}

type UpstreamResponse struct {
	ClaudeResponse
	ID   string `json:"id"`
	Type string `json:"type"`
}

func NewServer(port, upstream string, db *sql.DB) *Server {
	return &Server{
		port:     port,
		upstream: upstream,
		db:       db,
		server:   &http.Server{},
		mu:       sync.RWMutex{},
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", s.handleMessages)
	mux.HandleFunc("/v1/messages/", s.handleMessages)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","mnemo":"active"}`)
	})

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
	defer r.Body.Close()

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

	upstreamResp, err := s.forwardToUpstream(&req)
	if err != nil {
		log.Printf("Failed to forward to upstream: %v\n", err)
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}

	corrected := s.verifyResponse(upstreamResp.Content, project, query)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Mnemo-Corrected", fmt.Sprintf("%v", corrected != upstreamResp.Content))
	json.NewEncoder(w).Encode(ClaudeResponse{Content: corrected})
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
				return strings.Title(words[0])
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

	results, err := mnemodb.Search(searchQuery, 5)
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

func (s *Server) forwardToUpstream(req *ClaudeRequest) (*UpstreamResponse, error) {
	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	upstreamReq, err := http.NewRequest("POST", s.upstream+"/v1/messages", strings.NewReader(string(reqBody)))
	if err != nil {
		return nil, err
	}

	upstreamReq.Header.Set("Content-Type", "application/json")
	upstreamReq.Header.Set("anthropic-version", "2023-06-01")
	upstreamReq.Header.Set("x-api-key", os.Getenv("ANTHROPIC_API_KEY"))

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(upstreamReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var upstreamResp UpstreamResponse
	if err := json.Unmarshal(body, &upstreamResp); err != nil {
		return nil, err
	}

	return &upstreamResp, nil
}

func (s *Server) verifyResponse(content, project, query string) string {
	factChecks := []struct {
		trigger   string
		validator func(string) bool
		corrector func(string) string
	}{
		{
			trigger: "you previously",
			validator: func(s string) bool {
				return strings.Contains(s, "previously")
			},
			corrector: func(s string) string {
				corrected := getFactFromMnemo(project, query)
				if corrected != "" {
					return strings.Replace(s, "as of", "according to mnemo data")
				}
				return s
			},
		},
		{
			trigger: "as of",
			validator: func(s string) bool {
				return strings.Contains(s, "as of ")
			},
			corrector: func(s string) string {
				corrected := getFactFromMnemo(project, query)
				if corrected != "" {
					return strings.Replace(s, "as of", "according to mnemo data")
				}
				return s
			},
		},
	}

	corrected := content
	for _, check := range factChecks {
		if check.validator(corrected) {
			corrected = check.corrector(corrected)
		}
	}

	if corrected != content {
		log.Printf("Corrected hallucination in response (project: %s)\n", project)
	}

	return corrected
}

func getFactFromMnemo(project, query string) string {
	results, err := db.Search([]string{project}, 1)
	if err != nil || len(results) == 0 {
		return ""
	}

	return fmt.Sprintf("[mnemo data: %s]", results[0].Snippet)
}
