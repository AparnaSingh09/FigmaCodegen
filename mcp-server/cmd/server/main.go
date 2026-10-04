// Command server is the Figma-codegen MCP server: it exposes one tool,
// generate_code_from_figma_frame, that takes a pasted Figma frame link and
// returns generated code for it. Run over stdio - fine for local testing
// with the MCP Inspector, and for a client that spawns this as a
// subprocess; swap to an HTTP/SSE transport later if a remote client needs
// to connect over the network.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/AparnaSingh09/FigmaCodegen/mcp-server/internal/codegen"
	"github.com/AparnaSingh09/FigmaCodegen/mcp-server/internal/config"
	"github.com/AparnaSingh09/FigmaCodegen/mcp-server/internal/figma"
)

type generateArgs struct {
	FigmaURL  string `json:"figmaUrl" jsonschema:"the Figma share link for a specific frame (use Copy link to selection in Figma so it includes a node-id)"`
	Framework string `json:"framework,omitempty" jsonschema:"target framework for the generated code, e.g. react, html. Defaults to the server's configured default if omitted"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}
	if cfg.FigmaToken == "" {
		log.Fatal("FIGMA_TOKEN is not set - copy .env.example to .env and fill it in")
	}
	if cfg.GeminiAPIKey == "" {
		log.Fatal("GEMINI_API_KEY is not set - copy .env.example to .env and fill it in")
	}

	figmaClient := figma.NewClient(cfg.FigmaToken)

	ctx := context.Background()
	generator, err := codegen.NewGeminiGenerator(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("setting up gemini: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "figma-codegen", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "generate_code_from_figma_frame",
		Description: "Fetches a Figma frame (by its share link) and generates code that reproduces it.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args generateArgs) (*mcp.CallToolResult, any, error) {
		framework := args.Framework
		if framework == "" {
			framework = cfg.DefaultFramework
		}

		fileKey, nodeID, err := figma.ParseURL(args.FigmaURL)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid figma URL: %w", err)
		}

		nodeJSON, err := figmaClient.GetNodeJSON(fileKey, nodeID)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching frame data: %w", err)
		}

		image, err := figmaClient.GetImagePNG(fileKey, nodeID, cfg.MaxImageBytes)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching frame image: %w", err)
		}

		code, err := generator.GenerateCode(ctx, image, nodeJSON, framework)
		if err != nil {
			return nil, nil, fmt.Errorf("generating code: %w", err)
		}

		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: code}},
		}, nil, nil
	})

	log.Println("figma-codegen MCP server starting on stdio...")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}
