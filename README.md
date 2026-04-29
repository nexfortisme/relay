# Relay

Relay is a full-stack AI chat application with a Go backend and Vue 3 frontend. It supports streaming responses over WebSocket, tool calling through MCP, file attachments (including images), and local persistence with SQLite.

## Features

- Real-time AI chat with token-by-token streaming
- Optional extended "thinking" stream separate from final response text
- Conversation management: create, rename, archive, restore, delete
- Retry and stop controls for in-progress or failed assistant generations
- File attachments with vision-ready image handling
- OpenAI-compatible provider support (LM Studio, Ollama-compatible gateways, and others)
- MCP-based tool execution with built-in weather/search/time/fetch tools
- Optimistic frontend updates with reconciliation to server-persisted messages

## Architecture

```text
web (Vue 3 + Pinia + Vite)  <->  api (Gin + SQLite + WebSocket)
                                          |
                                          v
                              internal MCP server (:8090)
```

- `api/`: backend service, HTTP API, WebSocket stream, SQLite storage, LLM integration, MCP runtime
- `web/`: frontend app, Pinia store-driven state, streaming UI
- `scripts/dev.sh`: bootstraps dependencies and runs backend + frontend together

## Requirements

- Go (current stable)
- Bun
- Air (`go install github.com/air-verse/air@latest`)
- Node.js (used by frontend tooling; project targets modern Node versions)

## Quick Start

1. Copy environment variables:

   ```bash
   cp example.env .env
   ```

2. Configure at least:

   ```bash
   LLM_URL=http://localhost:1234/v1
   LLM_MODEL=your-model-name
   ```

3. Start both services:

   ```bash
   ./scripts/dev.sh
   ```

4. Open the app:

   - Frontend: `http://localhost:5173`
   - Backend API: `http://localhost:8091/api`

## Development Commands

### Run everything

```bash
./scripts/dev.sh
```

### Backend (`api/`)

```bash
go mod download
air
go build ./...
```

### Frontend (`web/`)

```bash
bun install
bun dev
bun test:unit
bun run lint
bun run type-check
bun run build
```

## Configuration

Key environment variables:

- `LLM_URL`: OpenAI-compatible API base URL
- `LLM_MODEL`: default model identifier
- `API_PORT`: backend HTTP port (default `8091`)
- `MCP_SERVER_ADDRESS`: internal MCP bind address (default `:8090`)
- `MCP_URL`: MCP endpoint used by runtime (default `http://localhost:8090/mcp`)
- `WEB_ORIGIN`: allowed web origin for CORS (default `http://localhost:5173`)
- `SQLITE_PATH`: SQLite file path
- `MAX_UPLOAD_BYTES`, `MAX_IMAGE_BYTES`: backend upload limits
- `VITE_MAX_UPLOAD_BYTES`, `VITE_MAX_IMAGE_BYTES`, `VITE_MAX_TOKEN_COUNT`: frontend limits

See `example.env` for the full list.

## API Surface

All routes are prefixed with `/api`.

- `GET/PUT /settings`
- `POST /conversations`
- `GET /conversations?includeArchived=1`
- `PATCH /conversations/:id`
- `PATCH /conversations/:id/archive`
- `PATCH /conversations/:id/restore`
- `DELETE /conversations/:id`
- `POST /conversations/:id/suggest-title`
- `GET /conversations/:id/messages`
- `POST /conversations/:id/messages`
- `POST /conversations/:id/messages/failed`
- `POST /conversations/:id/messages/:id/requeue`
- `GET /conversations/:id/stream` (WebSocket)
- `POST /conversations/:id/stop`

WebSocket event types:

- `token`
- `thinking`
- `done`
- `stopped`
- `error`
- `ping`
