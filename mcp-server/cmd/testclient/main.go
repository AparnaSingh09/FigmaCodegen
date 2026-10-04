// Command testclient is a minimal MCP client, used only to exercise the
// figma-codegen server by hand without the Node-based MCP Inspector
// (which needs a newer Node than this machine has). It spawns the server
// binary as a subprocess over stdio, calls
// generate_code_from_figma_frame, and prints the result - the same round
// trip the real web app's backend will do later, just from a CLI instead
// of a REST handler.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	serverPath := flag.String("server", "./server", "path to the built figma-codegen server binary")
	figmaURL := flag.String("url", "", "Figma frame share link (required)")
	framework := flag.String("framework", "", "target framework, e.g. react, html (optional - server default applies if omitted)")
	flag.Parse()

	if *figmaURL == "" {
		log.Fatal("-url is required")
	}

	ctx := context.Background()

	client := mcp.NewClient(&mcp.Implementation{Name: "testclient", Version: "v0.1.0"}, nil)

	transport := &mcp.CommandTransport{Command: exec.Command(*serverPath)}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatalf("connecting to server: %v", err)
	}
	defer session.Close()

	args := map[string]any{"figmaUrl": *figmaURL}
	if *framework != "" {
		args["framework"] = *framework
	}

	fmt.Println("calling generate_code_from_figma_frame ...")
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "generate_code_from_figma_frame",
		Arguments: args,
	})
	if err != nil {
		log.Fatalf("calling tool: %v", err)
	}

	if result.IsError {
		fmt.Println("--- tool returned an error ---")
	} else {
		fmt.Println("--- generated code ---")
	}
	for _, c := range result.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			fmt.Println(text.Text)
		}
	}
}
