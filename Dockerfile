# syntax=docker/dockerfile:1
# Backend-only image (the SPA ships separately to Cloudflare Pages; see ADR-0030).
# Multi-stage: build a static Go binary, run it on distroless. pgx + excelize are
# pure Go, so CGO is off and the binary runs on a scratch-class base.

# ---- build ----
FROM golang:1.26 AS build
WORKDIR /src

# module graph first for layer caching
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./
# -tags timetzdata embeds the zoneinfo DB so the binary needs no tzdata in the
# runtime image (distroless static has none). -trimpath + -s -w shrink it.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -tags timetzdata \
    -ldflags="-s -w" -o /out/balances ./cmd/balances

# ---- run ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/balances /app/balances
EXPOSE 8080
USER nonroot:nonroot
# migrations run via fly.toml release_command (`/app/balances migrate up`);
# the default command serves.
ENTRYPOINT ["/app/balances"]
CMD ["serve"]
