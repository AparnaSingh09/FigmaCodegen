// Command server is the web app's backend: it's a plain MCP *client* that
// connects once (at startup) to the figma-codegen MCP server over stdio,
// keeps that session open for the life of the process, and exposes it to
// the frontend as a single REST endpoint - POST /api/generate.
package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/AparnaSingh09/FigmaCodegen/webapp/backend/internal/config"
)

type generateRequest struct {
	FigmaURL  string `json:"figmaUrl"`
	Framework string `json:"framework,omitempty"`
}

type generateResponse struct {
	Code  string `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
}

type server struct {
	session     *mcp.ClientSession
	mu          sync.Mutex // the MCP go-sdk's docs don't guarantee CallTool is safe for concurrent use on one session, so serialize calls rather than assume
	frontendURL string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	// Resolve to an absolute path and run the subprocess with its *own*
	// directory as its working directory - otherwise it inherits this
	// process's cwd, and its internal findFileUpward(".env"/"config.json")
	// search starts from the wrong place and never finds mcp-server's files.
	absServerPath, err := filepath.Abs(cfg.MCPServerPath)
	if err != nil {
		log.Fatalf("resolving mcp server path %s: %v", cfg.MCPServerPath, err)
	}
	cmd := exec.Command(absServerPath)
	cmd.Dir = filepath.Dir(absServerPath)

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "figma-codegen-webapp", Version: "0.1.0"}, nil)
	transport := &mcp.CommandTransport{Command: cmd}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connecting to mcp server at %s: %v (did you `go build -o server ./cmd/server` inside mcp-server/ ?)", absServerPath, err)
	}
	defer session.Close()

	srv := &server{session: session, frontendURL: cfg.FrontendURL}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/generate", srv.handleGenerate)

	log.Printf("webapp backend listening on :%s (mcp server: %s, frontend: %s)", cfg.Port, cfg.MCPServerPath, cfg.FrontendURL)
	if err := http.ListenAndServe(":"+cfg.Port, srv.withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func (s *server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.frontendURL)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req generateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, generateResponse{Error: "invalid request body"})
		return
	}
	if req.FigmaURL == "" {
		respondJSON(w, http.StatusBadRequest, generateResponse{Error: "figmaUrl is required"})
		return
	}

	args := map[string]any{"figmaUrl": req.FigmaURL}
	if req.Framework != "" {
		args["framework"] = req.Framework
	}

	s.mu.Lock()
	result, err := s.session.CallTool(r.Context(), &mcp.CallToolParams{
		Name:      "generate_code_from_figma_frame",
		Arguments: args,
	})
	s.mu.Unlock()
	if err != nil {
		respondJSON(w, http.StatusBadGateway, generateResponse{Error: err.Error()})
		return
	}

	var text string
	for _, c := range result.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			text += t.Text
		}
	}
	if result.IsError {
		respondJSON(w, http.StatusBadGateway, generateResponse{Error: text})
		return
	}
	respondJSON(w, http.StatusOK, generateResponse{Code: text})
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
