# Relay Web

Vue 3 frontend for Relay. It provides the authenticated shell, realtime chat UI, file previews, feeds reader, notebooks workspace, CSV viewer, and settings screens.

## Stack

- Vue 3, TypeScript, Vite
- Pinia stores for auth, chat, conversations, notebooks, settings, and UI state
- Vue Router guarded by cookie-backed session auth
- Vitest plus Vue Test Utils for unit and component coverage
- Bun for installs and scripts

## Requirements

- Bun
- Node.js `^20.19.0 || >=22.12.0`

If your shell uses an older Node, run:

```bash
nvm use 24
```

## Development

Install dependencies:

```bash
bun install
```

Run the frontend dev server:

```bash
bun dev
```

The dev server expects the Go API at `http://localhost:8091/api` unless `VITE_API_BASE_DEV` is set.

For the full app stack from the repository root, prefer:

```bash
./scripts/dev.sh
```

## Scripts

```bash
bun dev                 # Vite dev server
bun test:unit --run     # unit/component tests
bun run type-check      # vue-tsc project check
bun run lint            # oxlint + eslint autofix
bun run build-only      # production frontend bundle only
bun run build           # type-check plus production bundle
```

## Source Map

- `src/views/`: route-level screens (`ChatView`, `FeedsView`, `NotebooksView`, auth screens, placeholders)
- `src/components/`: shared UI for the shell, composer, message list, settings, notebooks, and previews
- `src/stores/`: Pinia orchestration for API state, streaming reconciliation, auth, and notebooks
- `src/lib/`: API wrappers and pure helpers for markdown, upload validation, file types, CSV parsing, and formatting
- `src/__tests__/`: Vitest tests for components, stores, and helpers

## Environment

- `VITE_API_BASE_DEV`: API base during Vite dev, default `http://localhost:8091/api`
- `VITE_API_BASE`: API base for production builds, default `/api`
- `VITE_MAX_UPLOAD_BYTES`: composer upload limit hint
- `VITE_MAX_IMAGE_BYTES`: image upload/compression limit hint
- `VITE_MAX_TOKEN_COUNT`: UI token budget hint

These are usually loaded from the root `.env` when using `./scripts/dev.sh`.
