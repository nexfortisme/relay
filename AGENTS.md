# AGENTS.md

Guidance for AI coding agents working in this repository.

## Environment Notes

- The `gh` CLI may not be installed in local agent environments. Prefer plain `git` commands and the available GitHub connector/tooling when needed.
- Frontend tooling requires Node `^20.19.0 || >=22.12.0`. In local shells, run `nvm use 24` before Bun commands if the active Node is older or Bun has trouble loading frontend tooling.
- Use Bun for frontend dependency and script commands. The repo may also contain npm lockfile metadata, but Bun is the preferred package manager for agent workflows here.
- Do not commit generated `web/dist/` output unless the user explicitly asks for build artifacts.

## Project Overview

Relay is a full-stack AI chat application with a Go backend and Vue 3 frontend. It streams LLM responses over WebSocket, persists conversations in SQLite, supports file attachments, and exposes tool calling through the Model Context Protocol (MCP).

## Repository Structure

```text
relay/
├── api/          # Go backend: Gin, SQLite, WebSocket, MCP runtime
├── web/          # Vue 3 + TypeScript frontend: Vite, Pinia, Vitest
├── resources/    # Prompt/resource markdown files used by backend packages
├── scripts/      # Development helper scripts
└── example.env   # Environment variable template
```

## Common Commands

### Full Dev Stack

```bash
./scripts/dev.sh
```

Requires Go, Bun, and Air (`go install github.com/air-verse/air@latest`).

### Backend (`api/`)

```bash
go mod download
go test ./...
go build ./...
air
```

Run backend Go commands from `api/` unless a task says otherwise.

### Frontend (`web/`)

```bash
nvm use 24
bun install
bun dev
bun test:unit --run
bun run type-check
bun run lint
bun run build-only
```

`bun run build` runs type-check plus production build. Use `build-only` when type-check has already been run.

## Architecture Notes

### Backend Packages

| Package | Responsibility |
|---|---|
| `app/` | Server initialization, Gin engine setup, route registration, static UI serving |
| `attachments/` | Upload processing, document extraction, image compression/data URLs |
| `chat/` | Core conversation service, LLM generation, streaming, title handling |
| `config/` | Environment loading and defaults |
| `httpapi/` | HTTP handlers, multipart parsing, WebSocket upgrade |
| `llm/` | OpenAI-compatible provider client, SSE parsing, tool call orchestration |
| `mcp/` | Internal MCP server exposing built-in tools |
| `prompts/` | Prompt resource loading from `resources/api/` |
| `store/` | SQLite data access for conversations, messages, attachments, settings |
| `tools/` | Runtime abstractions and built-in/remote tool implementations |

The backend starts the main HTTP API on `:8091` by default and an internal MCP server on `:8090`. LLM tool calls flow through `chat.Service` → `tools.Runtime` → MCP/built-in tool implementations.

### Frontend Structure

- `src/stores/appStore.ts` owns API calls, conversation state, streaming reconciliation, settings, uploads, and theme state.
- `src/lib/` contains shared pure helpers such as API wrappers, file type handling, markdown rendering, message formatting, and upload validation.
- `src/components/` contains reusable UI pieces for the chat shell, composer, message list, sidebars, settings, icons, and file preview.
- `src/views/` contains route-level views.
- `src/__tests__/` contains Vitest component and unit tests.

## Core App Flow

1. The frontend creates an optimistic local user message with a `local-{timestamp}` ID.
2. It posts to `/api/conversations/:id/messages`, using JSON for text-only messages and `FormData` for attachments.
3. The backend persists the user message, creates an empty assistant message, and starts generation.
4. The frontend listens on `/api/conversations/:id/stream`.
5. WebSocket `token` and `thinking` events update the assistant message incrementally.
6. Terminal events (`done`, `stopped`, `error`) finalize UI state and token/timing metadata.
7. The frontend reconciles optimistic user messages with persisted server messages.

## API Surface

All API routes are prefixed with `/api`.

```text
GET/PUT  /settings
POST     /conversations
GET      /conversations?includeArchived=1
PATCH    /conversations/:id
PATCH    /conversations/:id/archive
PATCH    /conversations/:id/restore
DELETE   /conversations/:id
POST     /conversations/:id/suggest-title
GET      /conversations/:id/messages
POST     /conversations/:id/messages
POST     /conversations/:id/messages/failed
POST     /conversations/:id/messages/:messageId/requeue
GET      /conversations/:id/messages/:messageId/attachments/:attachmentIndex/download
GET      /conversations/:id/stream
POST     /conversations/:id/stop
```

WebSocket events include `token`, `thinking`, `done`, `stopped`, `error`, and `ping`.

## Configuration

Copy `example.env` to `.env` and set at least:

```bash
LLM_URL=http://localhost:1234/v1
LLM_MODEL=<model-name>
```

Important variables:

- `API_PORT`: backend HTTP port, default `8091`
- `WEB_ORIGIN`: allowed CORS origin, default `http://localhost:5173`
- `SQLITE_PATH`: SQLite database path
- `MCP_SERVER_ADDRESS`: internal MCP bind address, default `:8090`
- `MCP_URL`: MCP endpoint used by runtime, default `http://localhost:8090/mcp`
- `VITE_API_BASE_DEV`: frontend dev API base
- `VITE_API_BASE`: built frontend API base
- `VITE_MAX_UPLOAD_BYTES`, `VITE_MAX_IMAGE_BYTES`, `VITE_MAX_TOKEN_COUNT`: frontend limits

## Coding Guidelines

- Prefer small, pure helpers in `web/src/lib/` for reusable frontend formatting/validation logic.
- Keep Pinia store changes focused on state orchestration and API flow; avoid burying generic utility logic in the store.
- Keep backend resource loading independent of the process working directory. Go tests run with each package as the working directory.
- Preserve optimistic message behavior and per-conversation message caching when changing streaming or navigation code.
- When touching attachment handling, consider both frontend validation and backend multipart/storage/prompt processing.
- Avoid broad refactors in `store/` and `chat/` unless tests cover the persistence and stream behavior being changed.

## Testing Expectations

Before handing off meaningful changes, run the relevant checks:

```bash
cd api && go test ./...
cd web && nvm use 24 && bun test:unit --run
cd web && nvm use 24 && bun run type-check
cd web && nvm use 24 && bun run lint
```

For frontend production confidence, also run:

```bash
cd web && nvm use 24 && bun run build-only
```

Add or update tests near the behavior you change. Prefer focused tests for extracted helpers and integration-level component/store tests for streaming, attachments, and routing behavior.
