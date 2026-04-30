FROM oven/bun:1 AS web-builder
WORKDIR /build/web

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./
ARG VITE_API_BASE=/api
ENV VITE_API_BASE=${VITE_API_BASE}
RUN bun run build-only

FROM golang:1.25 AS api-builder
WORKDIR /build/api

COPY api/go.mod api/go.sum ./
RUN go mod download

COPY api/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/relay ./main.go

FROM debian:bookworm-slim AS runtime
WORKDIR /app

RUN mkdir -p /data

COPY --from=api-builder /out/relay /app/relay
COPY --from=web-builder /build/web/dist /app/web/dist
COPY resources /app/resources

ENV VITE_API_PORT=8091
ENV API_PORT=8091
ENV SQLITE_PATH=/data/relay.db
ENV WEB_ORIGIN=*

EXPOSE 8091 8090
VOLUME ["/data"]

CMD ["/app/relay"]
