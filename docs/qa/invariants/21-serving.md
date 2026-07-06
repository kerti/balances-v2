# Zone: SERVING

Single-origin static-serving contract — the Go binary serving the built SPA
bundle from `WEB_DIR` alongside `/api` in the production image (ADR-0030).
Distinct from PRESENTATION (the rendered SPA's own correctness) and CONTRACT
(the `/api` error envelope): this zone guards the **hosting seam** — how a raw
HTTP request for a path resolves to either a static file, the SPA shell, a 404,
or a sibling route handler. The defining risks are a **deep-link/refresh 404**
(a bookmarked client route, ADR-0025, must serve the shell), a **stale-chunk
mis-serve** (a missing content-hashed bundle must 404, not answer 200
text/html — the browser would fail to parse HTML as a module, and a CDN could
cache the shell under the chunk URL), and a **catch-all shadowing** the `/api`
or health routes. This serving path shipped two bugs in the same class —
`/assets/*` SPA-fallback (#190, then #241) — because the e2e harness serves the
SPA via Vite's dev server, whose own fallback masks the Go `spaHandler` the
image actually ships; this zone catalogs the contract those bugs violated and
the integration coverage now guarding it (#244). Code:
`internal/httpserver/server.go` (`spaHandler` + the `/*` mount in
`buildRouter`); proven by `spa_test.go` (handler in isolation) and
`serving_integration_test.go` (the real router, precedence included).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-SERVING-01 | Deep-link / refresh fallback: a request whose path matches no file on disk, and is not an extension-bearing `/assets/` request, resolves to `index.html`, so a bookmarked or hard-refreshed client route (ADR-0025) serves the SPA shell rather than 404 | ADR-0030, ADR-0025 | Medium |
| INV-SERVING-02 | Stale-chunk safety: a missing file under `/assets/` that names a file (has an extension) returns 404, never the SPA shell — a stale content-hashed chunk request must not get `200 text/html`, or the browser fails to parse the HTML as a module and a CDN can cache the shell under the chunk URL (#190; pairs with the #191 reload-on-chunk-error boundary) | ADR-0030 | High |
| INV-SERVING-03 | Extension disambiguation: an extensionless client route that collides with the build-output dir — the Assets section under `/assets/` (e.g. `/assets/bank-accounts`, App.tsx) — falls back to the SPA shell, not 404; only an extension-bearing `/assets/` miss 404s (the rule that lets INV-SERVING-02 and a client route coexist under the same prefix, #241) | ADR-0030 | Medium |
| INV-SERVING-04 | Route precedence: the SPA catch-all (`/*`) is mounted last and never shadows the sibling route trees — `/api/*` and `/healthz` resolve to their own handlers (or, for an unmounted `/api` path, the `/api` subtree's 404), never the SPA shell, in the single-origin image. This is the wiring a handler-in-isolation test cannot reach | ADR-0030 | High |
| INV-SERVING-05 | Traversal is rejected: a request path that escapes `WEB_DIR` (`..`) is refused (`http.ServeFile`'s guard / the within-root prefix check), never serving a file outside the served directory | ADR-0030 | High |
| INV-SERVING-06 | Every response (API and static) carries hardening headers — CSP (`default-src 'self'`, `frame-ancestors 'none'`, no `script-src 'unsafe-inline'`), `X-Content-Type-Options: nosniff`, and `Referrer-Policy`; `Strict-Transport-Security` is set only when `COOKIE_SECURE=true`, never over an assumed-plain-HTTP deployment | — | High |
