# AGENTS.md

Guidance for AI coding agents working in this repository.

## Environment Notes

- The `gh` CLI is not installed. Use plain `git` commands only. Never suggest or attempt `gh` commands.
- When asked to create a PR, **only push the branch** (`git push`). The user will open the PR manually in the browser.
- Frontend tooling requires Node `^20.19.0 || >=22.12.0`. In local shells, run `nvm use 24` before Bun commands if the active Node is older or Bun has trouble loading frontend tooling.
- Use Bun for frontend dependency and script commands.
- Do not commit generated `web/dist/` output unless the user explicitly asks for build artifacts.

## Project Overview

Relay is a full-stack AI chat application with a Go backend and Vue 3 frontend. It streams LLM responses over WebSocket, persists data in SQLite, supports multi-user authentication, file attachments, notebooks (RAG-style file grouping), RSS feeds with LLM summarization, and tool calling through the Model Context Protocol (MCP).

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

### Backend Packages (`api/internal/`)

| Package | Responsibility |
|---|---|
| `app/` | Server initialization, Gin engine setup, route registration, static UI serving |
| `auth/` | JWT-based authentication service, refresh tokens |
| `attachments/` | Upload processing, document extraction, image compression/data URLs |
| `chat/` | Core conversation service, LLM generation, streaming, title handling |
| `config/` | Environment loading and defaults |
| `feeds/` | RSS feed management, parsing, item summarization via LLM |
| `httpapi/` | HTTP handlers, multipart parsing, WebSocket upgrade (auth, conversations, messages, notebooks, feeds, settings) |
| `llm/` | OpenAI-compatible provider client, SSE parsing, tool call orchestration |
| `mcp/` | Internal MCP server exposing built-in tools |
| `notebooks/` | Notebook file management, chunking, CSV import, semantic search, PDF processing |
| `prompts/` | Prompt template loading from `resources/api/` markdown files |
| `store/` | SQLite data access for auth, conversations, messages, attachments, settings, notebooks, feeds |
| `tools/` | Runtime abstractions and built-in/remote tool implementations (Browse, Search, Time, Weather, Feed search) |

The backend starts the main HTTP API on `:8091` by default and an internal MCP server on `:8090`. LLM tool calls flow through `chat.Service → tools.CompositeRuntime → MCPRuntime → internal MCP server`.

### Frontend Structure

- Routed chat: `src/stores/chatStore.ts` (streaming, drafts, uploads, message caches) + `conversationStore.ts` (sidebar, URLs).
- `authStore.ts` — JWT session, user info, login/logout.
- `notebookStore.ts` — notebook management and file uploads.
- `settingsStore.ts` — user/system settings.
- `uiStore.ts` — modals, sidebars, UI chrome.
- `appStore.ts` — legacy consolidated store still referenced by Vitest suites; imports the same streaming helpers.
- `src/lib/conversationStreamMessages.ts` — centralizes merge logic between persisted REST messages and in-flight WebSocket state, optimistic user reconciliation, and stream payload typings.
- `src/lib/` — API wrappers, file handling, markdown rendering, upload validation, CSV parsing, notebook utilities.
- `src/components/` — chat shell, composer, message list, sidebars, settings, notebook panels, CSV viewer, file preview, icons.
- `src/views/` — `ChatView`, `FeedsView`, `HomeView`, `LandingView`, `LoginView`, `RegisterView`, `NotebooksView`, `MyDataView`, `ScheduledView`.
- `src/__tests__/` — Vitest component and unit tests.

## Core App Flow

1. The frontend creates an optimistic local user message with a `local-{timestamp}` ID.
2. It posts to `/api/conversations/:id/messages`, using JSON for text-only messages and `FormData` for attachments.
3. The backend persists the user message, creates an empty assistant message, and starts generation.
4. The frontend listens on `/api/conversations/:id/stream`.
5. WebSocket `token` and `thinking` events update the assistant message incrementally.
6. Terminal events (`done`, `stopped`, `error`) finalize UI state and token/timing metadata.
7. The frontend reconciles optimistic user messages with persisted server messages.

## API Surface

All routes are prefixed with `/api`.

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
PATCH  /conversations/:id
PATCH  /conversations/:id/archive
PATCH  /conversations/:id/restore
PATCH  /conversations/:id/favorite
DELETE /conversations/:id
POST   /conversations/:id/suggest-title
POST   /conversations/:id/stop
GET    /conversations/:id/stream                  # WebSocket
GET    /conversations/:id/messages
POST   /conversations/:id/messages
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

WebSocket events: `token`, `thinking`, `done`, `stopped`, `error`, `ping`.

## Configuration

Copy `example.env` to `.env` and set at least:

```bash
LLM_URL=http://localhost:1234/v1
LLM_MODEL=<model-name>
```

Important variables:

- `API_PORT`: backend HTTP port, default `8091`
- `WEB_ORIGIN`: allowed CORS origin, default `http://localhost:5173`
- `SQLITE_PATH`: main SQLite path (default `../.relay/data/relay.db` from `api/` cwd)
- `MCP_SERVER_ADDRESS`: internal MCP bind address, default `:8090`
- `MCP_URL`: MCP endpoint used by runtime, default `http://localhost:8090/mcp`
- `VITE_API_BASE_DEV` / `VITE_API_BASE`: frontend dev and built API base URLs
- `VITE_MAX_UPLOAD_BYTES`, `VITE_MAX_IMAGE_BYTES`, `VITE_MAX_TOKEN_COUNT`: frontend limits

## Coding Guidelines

- Prefer small, pure helpers in `web/src/lib/` for reusable frontend logic (streaming, formatting, validation).
- Keep Pinia store changes focused on state orchestration and API flow; avoid burying generic utility logic in stores.
- Keep backend resource loading independent of the process working directory — Go tests run with each package as the working directory.
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
