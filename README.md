# Relay

Relay is a full-stack AI application with a Go backend and Vue 3 frontend. The main experience is realtime chat with token-by-token streaming over WebSocket, SQLite persistence, and optional tool calling (Model Context Protocol plus first-party feed tools). A built-in **Feeds** inbox subscribes to RSS/Atom sources, polls on a schedule, and can summarize items with your configured LLM.

## Features

### Chat

- Streaming assistant output with incremental `token` events and optional separate `thinking` / reasoning stream
- Conversation CRUD: create, rename, archive, restore, delete
- Retry and stop for in-flight or failed assistant generations; requeue failed messages
- Attachments from the composer (including images suited for multimodal models) with optimistic UI that reconciles to server-backed messages
- Markdown rendering for assistant content in the UI
- Usage-style metadata where the model/provider exposes it (e.g. token counts)

### Feeds

- Subscribe to RSS/Atom URLs with a preview **check feed** flow before saving
- Configurable polling interval, automatic periodic refresh in the backend, manual refresh in the UI
- Backfill options when adding a feed (e.g. latest / since date / fuller history with limits)
- Read and starred state; unread, starred, and all-item views
- On-demand or automatic **LLM summaries** for items (automatic summary is gated on feed settings); expanded / re-summary actions in the UI
- Feed settings placeholder fields for routing items into **notebooks** when that area ships

### Dashboard and navigation

- **Home** launcher for quick jumps (e.g. open Feeds, start Chat with prefilled draft)
- **Chat** and **Feeds** are fully wired to the API
- **Notebooks**, **Scheduled**, and **My Data** routes exist with placeholder copy for future features

### Account and settings

- Cookie-based JWT access tokens with refresh tokens; register, login, logout, `/auth/me`
- Optional **`DISABLE_AUTH`** single-user mode (all requests treated as the seeded root user) for local dev
- Root user seeded from **`ROOT_USERNAME`** / **`ROOT_PASSWORD`** (change before any real deployment)
- Per-user persisted settings over the API: default LLM base URL, model id, optional API key, and custom **system prompt**

### Models and tools

- OpenAI-compatible chat completions (LM Studio, Ollama gateways, hosted APIs, etc.); **`LLM_URL`** or **`LLM_BASE_URL`**
- **Composite tool runtime**: tools are merged from the internal MCP HTTP server and from Relay’s feed tooling (`list_feeds`, `get_feed_items`), so the model can read your feed inbox inside chat
- Internal MCP tools (streamable HTTP on **`MCP_SERVER_ADDRESS`**, exposed at **`MCP_URL`**): `web_search` (SearxNG when configured), `get_weather`, `get_time`
- **`fetch_url`** / **`fetch_urls`** are registered on the MCP server but call an external **fetcher** MCP relay (`FETCHER_MCP_ENDPOINT`, default `http://localhost:3000/mcp`) for JS-heavy pages — run that stack if you rely on browsing-style fetch

### Operations

- **`GET /health`** on the main HTTP server for liveness checks
- When `web/dist` is present beside the backend, Gin serves static assets and **SPA fallback** for non-API routes (Docker / production-style single port)

## Architecture

```text
web (Vue 3 + Pinia + Vue Router + Vite)
        |  HTTP / WS
        v
api (Gin + SQLite + WebSocket chat + feed scheduler)
        |
        +-- internal MCP (:8090)  streamable HTTP tools
        |
        +-- optional FETCHER MCP (browse/fetch_url) via FETCHER_MCP_ENDPOINT
```

- **`api/`**: HTTP API, WebSocket stream, SQLite, LLM client, attachments, MCP client/runtime, RSS/Atom feed fetch + summarize
- **`web/`**: Pinia stores (auth, chat, conversations, settings, UI), routed views
- **`resources/`**: prompt and resource markdown used by backend packages
- **`scripts/dev.sh`**: installs deps and runs **`air`** for the API and **`bun dev`** for the frontend

## Requirements

- Go (current stable)
- Bun
- Air (`go install github.com/air-verse/air@latest`)
- Node.js `^20.19.0 || >=22.12.0` (e.g. `nvm use 24` before Bun commands if needed)

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
go test ./...
go build ./...
```

### Frontend (`web/`)

```bash
nvm use 24
bun install
bun dev
bun test:unit --run
bun run lint
bun run type-check
bun run build
# optional when type-check already ran:
bun run build-only
```

## Configuration

Key environment variables:

- **`LLM_URL`** or **`LLM_BASE_URL`**: OpenAI-compatible API base URL (one of them required)
- **`LLM_MODEL`**: default model id when settings do not override
- **`API_PORT`**: backend HTTP port (default `8091`)
- **`MCP_SERVER_ADDRESS`**: internal MCP bind address (default `:8090`)
- **`MCP_URL`**: MCP endpoint consumed by the chat runtime (default `http://localhost:8090/mcp`)
- **`FETCHER_MCP_ENDPOINT`**: URL of the relay-fetcher MCP server used by `fetch_url` / `fetch_urls` (default `http://localhost:3000/mcp`)
- **`SEARXNG_URL`**: base URL for a SearxNG instance; enables meaningful `web_search` results
- **`WEB_ORIGIN`**: browser origin allowed by CORS (default `http://localhost:5173`)
- **`VITE_API_BASE_DEV`**: frontend API base during Vite dev (default `http://localhost:8091/api`)
- **`VITE_API_BASE`**: frontend API base in production builds (default `/api`)
- **`SQLITE_PATH`**: SQLite database file path
- **`JWT_TOKEN`**, **`JWT_REFRESH_TOKEN`**: secrets for signing and hashing tokens
- **`DISABLE_AUTH`**: disable auth wall and act as root user
- **`ROOT_USERNAME`**, **`ROOT_PASSWORD`**: seeded admin account
- **`COOKIE_SECURE`**: require HTTPS for cookies when true
- **`VITE_MAX_UPLOAD_BYTES`**, **`VITE_MAX_IMAGE_BYTES`**, **`VITE_MAX_TOKEN_COUNT`**: limits mirrored in backend where applicable

See **`example.env`** for comments and defaults.

## Docker Deployment

Build an image that includes the compiled frontend and backend:

```bash
docker build -t relay:latest .
```

Run with SQLite persisted on a host volume:

```bash
docker run --rm -p 8091:8091 -v relay-data:/data \
  -e LLM_URL=http://host.docker.internal:1234/v1 \
  -e LLM_MODEL=your-model-name \
  relay:latest
```

Run in the background (detached) with `.env` and persistent SQLite:

```bash
docker run -d \
  --name relay \
  -p 8091:8091 \
  -v relay-data:/data \
  --env-file .env \
  -e SQLITE_PATH=/data/relay.db \
  --restart unless-stopped \
  relay:latest
```

Notes:

- The container defaults **`SQLITE_PATH`** to `/data/relay.db`.
- Mount `/data` so the database survives image rebuilds.
- The bundled UI is served from the backend; only **`8091`** (or your chosen **`API_PORT`**) needs to be published unless you terminate TLS elsewhere.

## API Surface

Routes under **`/api`** unless noted.

Auth (no prior session required):

- **`POST /auth/register`**
- **`POST /auth/login`**
- **`POST /auth/logout`**
- **`POST /auth/refresh`**

Authenticated:

- **`GET /auth/me`**
- **`GET` / `PUT /settings`** — per-user LLM URL, model, API key, system prompt

Conversations:

- **`POST /conversations`**
- **`GET /conversations`** — supports `includeArchived`
- **`PATCH /conversations/:id`**
- **`PATCH /conversations/:id/archive`**
- **`PATCH /conversations/:id/restore`**
- **`DELETE /conversations/:id`**
- **`POST /conversations/:id/suggest-title`**

Messages and streaming:

- **`GET /conversations/:id/messages`**
- **`POST /conversations/:id/messages`** — JSON body or multipart with `files[]`
- **`POST /conversations/:id/messages/failed`**
- **`POST /conversations/:id/messages/:messageId/requeue`**
- **`GET /conversations/:id/stream`** — WebSocket upgrade
- **`POST /conversations/:id/stop`**

Files:

- **`GET /files/:id/download`** — attachment bytes for authenticated owner

Feeds:

- **`GET /feeds`**
- **`POST /feeds/check`** — resolve and describe a subscription URL without persisting
- **`POST /feeds`**
- **`GET /feeds/items`** — list items; query **`view`** = `unread` (default), `starred`, or `all`; optional **`feedId`** to scope one subscription
- **`GET /feeds/items/:id`**
- **`PATCH /feeds/items/:id`** — e.g. read / starred toggles
- **`POST /feeds/items/:id/summarize`**
- **`PATCH /feeds/:id`**
- **`DELETE /feeds/:id`**

Other:

- **`GET /health`** — process liveness (not under `/api`)

Except for the auth bootstrap routes above, **`/api/*`** routes expect a valid session cookie (or **`DISABLE_AUTH`**).

### WebSocket event types

- `token`
- `thinking`
- `done`
- `stopped`
- `error`
- `ping`
