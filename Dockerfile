# syntax=docker/dockerfile:1
# Single-origin image (ADR-0030): one app serves the built SPA + the /api
# backend. Three stages — build the SPA, build the Go binary, assemble a
# distroless runtime with both. pgx + excelize are pure Go, so CGO is off and
# the binary runs on a scratch-class base.

# ---- web (SPA) ----
FROM node:22 AS web
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build   # -> /web/dist

# ---- build (Go) ----
FROM golang:1.26 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# -tags timetzdata embeds the zoneinfo DB (distroless static has none).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -tags timetzdata \
    -ldflags="-s -w" -o /out/balances ./cmd/balances

# ---- run ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/balances /app/balances
COPY --from=web /web/dist /app/web
EXPOSE 8080
USER nonroot:nonroot
# WEB_DIR (set in fly.toml) makes the binary serve /app/web alongside /api.
# Migrations run via the fly.toml release_command; the default command serves.
ENTRYPOINT ["/app/balances"]
CMD ["serve"]
