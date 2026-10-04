# FigmaCodegen

An app that reads a Figma design and generates code for it — built mainly
as a learning project for **how MCP (Model Context Protocol) actually
works**, by building our own MCP server rather than relying on Figma's
official one (which is paid-seat-gated and locked to an allowlist of
clients like VS Code/Cursor/Claude Code).

## How it's structured

Two pieces:

1. **`mcp-server/`** — our own MCP server. Wraps Figma's open REST API
   (not the gated Dev Mode MCP server) plus a vision-capable LLM, exposed
   as one MCP tool: `generate_code_from_figma_frame`. This is the actual
   MCP-learning piece — server, tool schema, transport.
2. **`webapp/`** (not built yet) — a small web app (Go backend as an MCP
   *client* + a React frontend) so people can paste a Figma link into a
   browser UI instead of using an MCP-aware editor.

```
FigmaCodegen/
  mcp-server/
    cmd/server/main.go        — entrypoint, registers the tool, runs over stdio
    internal/config/          — .env (secrets) + config.json (settings) split
    internal/figma/           — Figma REST client (node data + rendered image)
    internal/codegen/         — pluggable CodeGenerator interface + Gemini impl
    config.json
    .env.example
  webapp/                      — TODO
```

## Flow

```mermaid
sequenceDiagram
    participant U as User (browser)
    participant FE as Web frontend
    participant BE as Web backend (MCP client)
    participant MCP as mcp-server (MCP tool)
    participant Figma as Figma REST API
    participant Gemini as Gemini (vision LLM)

    U->>FE: Paste Figma frame link + pick framework
    FE->>BE: POST /api/generate {figmaUrl, framework}
    BE->>MCP: call tool generate_code_from_figma_frame
    MCP->>Figma: GET /v1/files/:key/nodes (structured data)
    Figma-->>MCP: layout, text, styles (JSON)
    MCP->>Figma: GET /v1/images/:key (render request)
    Figma-->>MCP: rendered PNG
    MCP->>Gemini: image + structured data + framework
    Gemini-->>MCP: generated code
    MCP-->>BE: tool result (code)
    BE-->>FE: {code}
    FE-->>U: show code in UI
```

`webapp/` (frontend + backend/MCP client) doesn't exist yet - today you can
exercise everything from "call tool" onward directly via the MCP Inspector,
without the top two rows.

## mcp-server: how it works

Given a Figma frame link (e.g. `https://www.figma.com/design/ABC123/My-File?node-id=1-23`),
the `generate_code_from_figma_frame` tool:

1. Parses the file key + node ID out of the URL.
2. Fetches the node's structured data (`GET /v1/files/:key/nodes`) — layout,
   text, styles.
3. Fetches a rendered PNG of the frame (`GET /v1/images/:key`).
4. Sends both (image + structured data) to Gemini, asking for code in the
   requested framework (default: React).
5. Returns the generated code as the tool's result.

Auth note: Figma's REST API takes personal access tokens via a custom
`X-Figma-Token` header — **not** `Authorization: Bearer` (that's for OAuth
tokens only, and silently 403s a PAT).

### Config

Same split as commit-notification-app, for the same reason — secrets never
belong in a committed file:

| File | Contents |
|---|---|
| `.env` (gitignored, copy from `.env.example`) | `FIGMA_TOKEN`, `GEMINI_API_KEY` |
| `config.json` (committed) | `geminiModel`, `defaultFramework`, `maxImageBytes` |

### Running it

```bash
cd mcp-server
cp .env.example .env   # fill in FIGMA_TOKEN and GEMINI_API_KEY
go build ./cmd/server
```

Test it standalone with the official MCP Inspector before wiring up any
client:

```bash
npx @modelcontextprotocol/inspector ./server
```

### LLM provider

Currently Gemini (`gemini-2.5-flash`) — picked for its free tier with real
vision support, needed since this task requires the model to actually look
at the rendered frame. `CodeGenerator` is an interface
(`internal/codegen/codegen.go`), so other vision-capable providers (Claude,
etc.) can be added the same way `groq`/`claude` were pluggable in
commit-notification-app.

## webapp (planned)

- Backend: Go, acts as the MCP *client* — connects to `mcp-server` and
  calls its tool, exposes one REST endpoint for the frontend
  (`POST /api/generate { figmaUrl, framework }`).
- Frontend: React/Vite — paste a link, pick a framework, see the generated
  code.

No auth/DB needed for this — it's a personal learning project, not a
multi-user product.

## Do we need an "agent" here?

No, not for the current one-tool flow — there's only one tool and no
decision to make about which one to call or in what order, so the web
backend is a plain MCP client, not an agent. An agent would earn its keep
if this grows into several smaller tools (e.g. `list_frames`,
`get_frame_image`, `generate_code`) with an LLM deciding which to call and
when — a possible stretch goal, not part of the MVP.
