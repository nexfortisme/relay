# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Environment Notes

- `gh` CLI is not installed — avoid GitHub CLI commands. NEVER SUGGEST USING `gh` OR ANY `gh` COMMANDS.
- Frontend tooling requires Node `^20.19.0 || >=22.12.0`. Run `nvm use 24` before Bun commands if the active Node is older or Bun has trouble loading frontend tooling.
- Use Bun for frontend dependency and script commands.

## Overview

Relay is a full-stack AI chat application — Go backend + Vue 3 frontend — that streams LLM responses over WebSocket and executes tools via the Model Context Protocol (MCP).

## Repository Structure

```
relay/
├── api/          # Go backend (Gin + SQLite + WebSocket)
├── web/          # Vue 3 + TypeScript frontend (Vite + Pinia)
├── scripts/      # dev.sh — starts both services concurrently
└── example.env   # All required environment variables with descriptions
```

## Commands

### Development (both services)
```bash
./scripts/dev.sh   # requires Go, Bun, and Air (go install github.com/air-verse/air@latest)
```

### Frontend (`web/`)
```bash
nvm use 24        # ensure compatible Node for Bun/Vite tooling
bun install        # install dependencies
bun dev            # Vite dev server on :5173
bun test:unit --run # Vitest once
bun run lint       # ESLint + Oxlint
bun run type-check # vue-tsc
bun run build      # production build → web/dist/
```

### Backend (`api/`)
```bash
go mod download    # fetch dependencies
air                # hot-reload dev server on :8091
go build           # standard Go build
```

## Architecture

### Backend (`api/`)

| Package | Responsibility |
|---|---|
| `app/` | Server init, Gin engine, route registration |
| `chat/` | Core service: LLM calls, tool execution, streaming |
| `store/` | SQLite DAL (conversations, messages, attachments, settings) |
| `httpapi/` | HTTP handlers + WebSocket upgrade |
| `llm/` | OpenAI-compatible provider client |
| `tools/` | Tool runtime abstraction + built-ins (Weather, Search, Time, FetchURL) |
| `mcp/` | Internal MCP server that exposes tools |
| `attachments/` | File upload processing, image→data-URL conversion |
| `config/` | Env-based configuration |

The backend runs two servers: the main HTTP API on `:8091` and an internal MCP server on `:8090`. When the LLM requests tool use, the chat service calls `CompositeRuntime → MCPRuntime → internal MCP server`.

### Frontend (`web/src/`)

State is split across Pinia stores: `stores/chatStore.ts` and `stores/conversationStore.ts` own realtime chat/streaming flows; authentication, notebooks, settings, and UI chrome live in sibling stores (`authStore`, `notebookStore`, `settingsStore`, `uiStore`). `stores/appStore.ts` remains an older consolidated store referenced by Vitest suites. Shared optimistic/stream merge helpers for WebSocket deltas live in `src/lib/conversationStreamMessages.ts`.

Key data flow for a user message:
1. Optimistic UI: local message with `local-{timestamp}` ID added immediately
2. POST to `/api/conversations/:id/messages` (JSON, or FormData if files attached)
3. Backend queues generation; frontend opens WebSocket at `/api/conversations/:id/stream`
4. `token` / `thinking` events update message content in real-time
5. `done` event closes streaming; local ID is reconciled with persisted server ID

### REST API Routes (all prefixed `/api`)

```
GET/PUT  /settings
POST     /conversations
GET      /conversations?includeArchived=1
PATCH    /conversations/:id                       # rename
PATCH    /conversations/:id/archive|restore
DELETE   /conversations/:id
POST     /conversations/:id/suggest-title
GET      /conversations/:id/messages
POST     /conversations/:id/messages              # create (JSON or FormData)
POST     /conversations/:id/messages/failed       # persist failed local message
POST     /conversations/:id/messages/:id/requeue  # retry generation
GET      /conversations/:id/stream                # WebSocket
POST     /conversations/:id/stop
```

### WebSocket Event Types

```json
{ "type": "token",    "messageId": "...", "token": "..." }
{ "type": "thinking", "messageId": "...", "thinking": "..." }
{ "type": "done",     "messageId": "...", "elapsedMs": 2500 }
{ "type": "stopped",  "messageId": "..." }
{ "type": "error",    "error": "..." }
{ "type": "ping" }
```

## Environment Configuration

Copy `example.env` and set at minimum:

```bash
LLM_URL=http://localhost:1234/v1   # any OpenAI-compatible endpoint
LLM_MODEL=<model-name>
```

Key variables:
- `API_PORT` — backend port (default `8091`)
- `WEB_ORIGIN` — CORS allowed origin (default `http://localhost:5173`)
- `SQLITE_PATH` — main SQLite file (default `../.relay/data/relay.db` relative to API working directory — repo `.relay/` when using `./scripts/dev.sh` or `air` from `api/`)
- `MCP_SERVER_ADDRESS` / `MCP_URL` — internal MCP server (default `:8090` / `http://localhost:8090/mcp`)
- `VITE_API_BASE` — frontend API base URL (default `http://localhost:8091/api`)

## Key Patterns

- **Optimistic updates**: frontend assigns `local-{timestamp}` IDs and reconciles after the server persists.
- **Message caching**: the store caches messages per conversation so streaming state survives navigation between conversations.
- **Pluggable LLM**: any OpenAI-compatible endpoint works; model and system prompt are runtime settings stored in SQLite.
- **Extended thinking**: `thinking` field on messages stores reasoning tokens separately from response content.
- **File uploads**: validated on the frontend (single file ≤ 50 MB, images ≤ 15 MB), sent as `multipart/form-data`, stored as blobs in SQLite, and converted to data URLs before being sent to the LLM for vision tasks.
