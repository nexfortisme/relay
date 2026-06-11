# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Environment Notes

- `gh` CLI is not installed — avoid GitHub CLI commands. NEVER SUGGEST USING `gh` OR ANY `gh` COMMANDS.
- Frontend tooling requires Node `^20.19.0 || >=22.12.0`. Run `nvm use 24` before Bun commands if the active Node is older or Bun has trouble loading frontend tooling.
- Use Bun for frontend dependency and script commands.

## Pull Requests

When the user runs `/create-pr` or asks to create a PR, **only push the branch** with `git push`. Do not attempt to open a PR via `gh` or any other CLI — the user will create the PR manually in the browser.

## Overview

Relay is a full-stack AI chat application — Go backend + Vue 3 frontend — that streams LLM responses over WebSocket, persists data in SQLite, supports multi-user auth, file attachments, notebooks, RSS feeds, and tool calling via the Model Context Protocol (MCP).

## Repository Structure

```
relay/
├── api/          # Go backend (Gin + SQLite + WebSocket)
├── web/          # Vue 3 + TypeScript frontend (Vite + Pinia)
├── resources/    # Prompt/resource markdown files used by backend packages
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
nvm use 24         # ensure compatible Node for Bun/Vite tooling
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
go test ./...      # run all Go tests
go build           # standard Go build
```

## Architecture

### Backend (`api/internal/`)

| Package | Responsibility |
|---|---|
| `app/` | Server init, Gin engine, route registration, static UI serving |
| `auth/` | JWT-based authentication service |
| `attachments/` | File upload processing, document extraction, image compression/data URLs |
| `chat/` | Core service: LLM calls, tool execution, streaming, title generation |
| `config/` | Env-based configuration |
| `feeds/` | RSS feed management, parsing, item summarization |
| `httpapi/` | HTTP handlers + WebSocket upgrade (auth, conversations, messages, notebooks, feeds, settings) |
| `llm/` | OpenAI-compatible provider client, SSE parsing, tool call orchestration |
| `mcp/` | Internal MCP server that exposes tools |
| `notebooks/` | Notebook file management, chunking, CSV import, search, PDF processing |
| `prompts/` | Prompt template loading from `resources/api/` |
| `store/` | SQLite DAL (conversations, messages, attachments, auth, settings, notebooks, feeds) |
| `tools/` | Tool runtime abstraction + built-ins (Weather, Search, Time, FetchURL, Feed search, Browse) |

The backend runs two servers: the main HTTP API on `:8091` and an internal MCP server on `:8090`. LLM tool calls flow through `chat.Service → tools.CompositeRuntime → MCPRuntime → internal MCP server`.

### Frontend (`web/src/`)

State is split across Pinia stores:
- `chatStore.ts` + `conversationStore.ts` — real-time chat, streaming, message caches, optimistic updates
- `authStore.ts` — authentication, session, user info
- `notebookStore.ts` — notebook management and file uploads
- `settingsStore.ts` — user/system settings
- `uiStore.ts` — UI chrome (modals, sidebars)

Shared optimistic/stream merge helpers for WebSocket deltas live in `src/lib/conversationStreamMessages.ts`.

Views: `ChatView`, `FeedsView`, `HomeView`, `LandingView`, `LoginView`, `RegisterView`, `NotebooksView`, `MyDataView`, `ScheduledView`.

Key data flow for a user message:
1. Optimistic UI: local message with `local-{timestamp}` ID added immediately
2. POST to `/api/conversations/:id/messages` (JSON, or FormData if files attached)
3. Backend queues generation; frontend opens WebSocket at `/api/conversations/:id/stream`
4. `token` / `thinking` events update message content in real-time
5. `done` event closes streaming; local ID is reconciled with persisted server ID

### REST API Routes (all prefixed `/api`)

**Auth (public)**
```
POST /auth/register
POST /auth/login
POST /auth/logout
POST /auth/refresh
```

**Auth (authenticated)**
```
GET  /auth/me
```

**Settings**
```
GET  /settings
PUT  /settings
```

**Conversations**
```
POST   /conversations
GET    /conversations?includeArchived=1
PATCH  /conversations/:id                         # rename
PATCH  /conversations/:id/archive|restore|favorite
DELETE /conversations/:id
POST   /conversations/:id/suggest-title
POST   /conversations/:id/stop
GET    /conversations/:id/stream                  # WebSocket
GET    /conversations/:id/messages
POST   /conversations/:id/messages                # JSON or FormData
POST   /conversations/:id/messages/failed
POST   /conversations/:id/messages/:messageId/requeue
```

**Files**
```
GET /files/:id/download
```

**Notebooks**
```
POST   /notebooks
GET    /notebooks
GET    /notebooks/:notebookId
PATCH  /notebooks/:notebookId
DELETE /notebooks/:notebookId
POST   /notebooks/:notebookId/files
GET    /notebooks/:notebookId/files
DELETE /notebooks/:notebookId/files/:fileId
GET    /notebooks/:notebookId/files/:fileId/download
GET    /notebooks/:notebookId/files/:fileId/pages/:pageNum/image
GET    /notebooks/:notebookId/jobs/count
GET    /notebooks/:notebookId/conversations
POST   /notebooks/:notebookId/conversations
GET    /notebooks/:notebookId/csv/:fileId
```

**Feeds**
```
GET    /feeds
POST   /feeds/check
POST   /feeds
GET    /feeds/items?view=&feedId=
GET    /feeds/items/:id
PATCH  /feeds/items/:id
POST   /feeds/items/:id/summarize
POST   /feeds/:id/mark-read
PATCH  /feeds/:id
DELETE /feeds/:id
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
- `SQLITE_PATH` — main SQLite file (default `../.relay/data/relay.db` relative to API working directory)
- `MCP_SERVER_ADDRESS` / `MCP_URL` — internal MCP server (default `:8090` / `http://localhost:8090/mcp`)
- `VITE_API_BASE` / `VITE_API_BASE_DEV` — frontend API base URL
- `VITE_MAX_UPLOAD_BYTES`, `VITE_MAX_IMAGE_BYTES`, `VITE_MAX_TOKEN_COUNT` — frontend upload limits

## Key Patterns

- **Optimistic updates**: frontend assigns `local-{timestamp}` IDs and reconciles after the server persists.
- **Message caching**: the store caches messages per conversation so streaming state survives navigation.
- **Pluggable LLM**: any OpenAI-compatible endpoint works; model and system prompt are runtime settings stored in SQLite.
- **Extended thinking**: `thinking` field on messages stores reasoning tokens separately from response content.
- **File uploads**: validated on the frontend (single file ≤ 50 MB, images ≤ 15 MB), sent as `multipart/form-data`, stored as blobs in SQLite, converted to data URLs for vision tasks.
- **Notebooks**: group files (PDFs, CSVs, docs) for RAG-style retrieval in linked conversations.
- **Feeds**: RSS feed subscriptions with per-item LLM summarization.
- **Auth**: JWT with refresh tokens; optional auth mode configurable via settings.
