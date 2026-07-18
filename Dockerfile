# syntax=docker/dockerfile:1
# Single-origin image (ADR-0030): one app serves the built SPA + the /api
# backend. Three stages — build the SPA, build the Go binary, assemble a
# distroless runtime with both. pgx + excelize are pure Go, so CGO is off and
# the binary runs on a scratch-class base.

# ---- web (SPA) ----
# Built on the native build platform — the SPA bundle is architecture-agnostic,
# so there's no reason to emulate the target arch here.
FROM --platform=$BUILDPLATFORM node:26@sha256:0473e7dc433a1310f436edee02aa79737ec78a4b345433ab0963d4a256f9ad85 AS web
WORKDIR /web
# Release tag baked into the bundle (issue #75). The deploy workflow passes it
# as a build arg; Vite picks up VITE_*-prefixed vars from the environment at
# `npm run build`. Local builds use the default. The deploy target (DEPLOY_ENV)
# is NOT baked in here — it's read at runtime (#354), since the same built
# image is deployed to every environment.
ARG APP_VERSION=dev
ENV VITE_APP_VERSION=$APP_VERSION
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build   # -> /web/dist

# ---- build (Go) ----
# Run the toolchain on the native build platform and cross-compile to the
# target arch (TARGETOS/TARGETARCH are supplied by buildx per --platform). Go
# cross-compiles cleanly with CGO off, so this is far faster than emulating the
# target under QEMU, and it produces a multi-arch image (amd64 + arm64).
FROM --platform=$BUILDPLATFORM golang:1.26@sha256:079e59808d2d252516e27e3f3a9c003740dee7f75e55aa71528766d52bcfc16a AS build
ARG TARGETOS
ARG TARGETARCH
# Same release tag as the SPA's VITE_APP_VERSION (issue #75), baked into the Go
# binary so /healthz can report exactly what rolled out (#355) and the PDF
# report footer carries the deployed version (#414).
ARG APP_VERSION=dev
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# -tags timetzdata embeds the zoneinfo DB (distroless static has none).
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -tags timetzdata \
    -ldflags="-s -w -X github.com/kerti/balances-v2/backend/internal/version.Version=${APP_VERSION}" \
    -o /out/balances ./cmd/balances

# ---- run ----
FROM gcr.io/distroless/static-debian12:nonroot@sha256:b7bb25d9f7c31d2bdd1982feb4dafcaf137703c7075dbe2febb41c24212b946f
WORKDIR /app
COPY --from=build /out/balances /app/balances
COPY --from=web /web/dist /app/web
# Third-party license notices travel with the artifact they cover (issue #345):
# the aggregate for the Go binary + SPA bundle, plus the curated font OFL texts.
COPY THIRD-PARTY-NOTICES /app/THIRD-PARTY-NOTICES
COPY frontend/licenses /app/licenses
EXPOSE 8080
USER nonroot:nonroot
# WEB_DIR (set in fly.toml) makes the binary serve /app/web alongside /api.
# Migrations run via the fly.toml release_command; the default command serves.
ENTRYPOINT ["/app/balances"]
CMD ["serve"]
