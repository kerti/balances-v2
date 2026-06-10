# Hosting and deployment for alpha

This ADR closes the hosting question deferred in [[adr-0013]]. For alpha the stack runs on
**managed free/near-free tiers**: the React SPA **served by the Go app** on **Fly.io**,
PostgreSQL on **Neon**, and mail on **Resend** (already chosen in [[adr-0020]]). Deployment is
**tag-driven** off the release tags from [[adr-0029]]. Only **one environment is deployed at the
start — `preview`**; the developer's own machine serves as `dev` via a Cloudflare Tunnel, and
`demo`/`production` are provisioned only when a real RC / production ship exists.

## Why now

[[adr-0013]] packaged the app as portable Docker and explicitly deferred the hosting target "until
there's something deployable," foreseeing "Cloud Run / Render / Koyeb for backend; Neon / Supabase
for DB; Vercel / Cloudflare Pages for frontend." Alpha is that moment. The targets below are picked
to honour ADR-0013's portability constraint: each is consumed as a generic primitive (a container,
a Postgres connection string, a static-file host, an SMTP endpoint) with **no vendor SDK coupling**,
so moving off any one of them is a configuration change, not a rewrite.

## The decision

### Environments — start with one deployed

| Env | Where | Purpose | Status |
|---|---|---|---|
| `dev` | the maintainer's laptop, exposed via Cloudflare Tunnel | personal sandbox; unstable, stale data, missed migrations expected | **laptop, not deployed** |
| `preview` | Fly + Neon + CF Pages | alpha releases | **deployed now** |
| `demo` | Fly + Neon + CF Pages | beta / RC | **deferred** to first RC |
| `production` | Fly + Neon + CF Pages | real users | **deferred** |

Provisioning all four now is rejected as premature surface for a pre-alpha solo project — each env
is a subdomain + DB + Fly app + OAuth client + secrets + mail config to maintain. `demo` and
`production` are stood up the day they are first needed, reusing the same wiring.

### `dev` is the laptop behind a Cloudflare Tunnel

- `cloudflared` runs in the **foreground** (a `make` target), mapping
  `dev.<personal-domain>` → local `vite` / API ports through a persistent outbound connection —
  no inbound ports, no public IP.
- This gives a **stable hostname** for the Google OAuth redirect URI and lets other devices (phone)
  reach the running local instance, which `localhost` cannot. The OAuth "redirect limitation" was
  really "other devices can't resolve `localhost`"; a stable host fixes it.
- Availability is intentionally laptop-bound: tunnel down when the laptop sleeps or `cloudflared`
  stops. This matches the "unstable, stale, personal" intent — `dev` *is* local, not a service.
- It is single-user. Other contributors run their own local `dev`; there is no shared dev service.
- The tunnel serves whatever local data is loaded. Reset/reseed scripts keep it sane and are the
  hygiene path before showing `dev` to anyone (and align with the rule against exposing real test
  data).

### Targets

- **SPA → served by the Go app (single image).** The frontend is built (`vite build`) into the same
  Docker image and served by the Go binary alongside `/api` — one origin, one deploy, no separate
  static host. Cloudflare can sit *in front* at demo/production for CDN/WAF (see the single-origin
  section); it is not needed for `preview`.
- **Go API → Fly.io.** Deploys the ADR-0013 Docker image directly. Chosen over Koyeb specifically
  for **`release_command`**, which runs `goose up` pre-rollout and **blocks the deploy if the
  migration fails** — the deciding factor for a DB-backed app. Pay-as-you-go per-machine billing
  with **scale-to-zero** keeps an idle `preview` at pennies and makes adding `demo`/`production`
  just another app on the same model.
- **PostgreSQL → Neon.** Free tier that *persists* (unlike Render's withdrawn free PG), scale-to-
  zero, and **database branching** — one Neon project with a branch per environment instead of N
  separate databases to babysit. Treated strictly as a Postgres connection string per ADR-0013; no
  Neon-specific extensions.
- **Mail → Resend.** Already adopted in [[adr-0020]]; Mailpit remains local-dev only, so every
  non-local env needs a real SMTP/API path. Resend's free tier covers alpha volume.

### Single origin: the Go app serves the SPA and `/api`

Each environment is **one origin** — one Fly app serves both the built SPA and the API. The backend
mounts the API under `r.Route("/api", …)` (`internal/httpserver/server.go`, plus a top-level
`/healthz`); when `WEB_DIR` is set it also serves the static SPA from that directory, falling back to
`index.html` for client-side routes ([[adr-0025]]). In dev `WEB_DIR` is unset — Vite serves the SPA
and proxies `/api` to the backend.

This is deliberate, not cosmetic. Auth is **server-side session cookies** ([[adr-0017]]); the session
cookie is **host-only** (no `Domain`), `HttpOnly`, `Secure` in prod, `SameSite=Lax`. A single origin
keeps it **first-party** — a split SPA-host / API-host layout would force `SameSite=None; Secure`
cross-site cookies, which Safari ITP and Chrome's third-party-cookie phase-out drop, silently
breaking login. One origin sidesteps that at every tier and keeps the domain swappable (host-only
cookie + env-driven URLs, no rebuild).

`preview` runs as the bare Fly hostname (`balances-preview.fly.dev`). `demo`/`production` add
**Cloudflare *in front*** of the same Fly app (proxied custom domain) for TLS edge, CDN, and WAF — a
cache rule on Vite's hashed `/assets/*` recovers edge caching. Purely additive: same image, same
single origin, same cookie/OAuth flow; only a DNS/proxy layer is added. (When Cloudflare fronts the
app it becomes a trusted proxy, enabling real-client-IP extraction — currently off, see `server.go`.)

Per-env config: `COOKIE_SECURE=true`, and `FRONTEND_URL` / `BACKEND_URL` / `OAUTH_REDIRECT_URL` point
at that env's hostname.

### Tag-driven deployment

A single `.github/workflows/deploy.yml` triggers on `v*` tag push and routes by tag shape:

```
v0.6.0-alpha.N   → preview     (auto)
v0.6.0-rc.N      → demo        (auto, once demo exists)
v0.6.0           → production  (manual approval via GitHub Environment)
```

- Secrets and per-env config live in **GitHub Environments** (`preview` / `demo` / `production`).
  The `production` environment carries a **required reviewer**, adding a second hard gate on top of
  the merge signoff from [[adr-0029]].
- A single `flyctl deploy` against the env's Fly app builds the image (SPA + binary) and rolls it
  out; `goose up` runs inside Fly as the `release_command`. A Neon branch can dry-run a migration
  before it reaches a long-lived env. There is **no separate frontend deploy** — the SPA ships in the
  image, so the only GitHub secret per env is `FLY_API_TOKEN`.
- Each env has its own Google OAuth client / redirect URI and its own Resend + DB secrets.

## Considered alternatives

- **Koyeb for the API.** Rejected — no clean pre-deploy hook, so migrations would ride the start
  command (runs every boot, races on scale-up); free tier caps at one service, so `demo`/`production`
  force payment or juggling anyway. Fly's `release_command` and per-machine billing win. Koyeb
  remains the fallback if Fly changes terms.
- **A deployed `dev` environment.** Rejected for now — the laptop tunnel matches the stated
  "unstable / stale / personal" intent at zero infra. Revisit only if `dev` must be reachable while
  the laptop is off.
- **Render.** Rejected — withdrew its free Postgres tier and cold-starts free web services.
- **Provisioning all four environments now.** Rejected — premature maintenance surface before a
  single alpha user; `demo`/`production` are stood up on first real need.
- **One VPS running the whole stack** (a deferred option in ADR-0013). Rejected for alpha — more ops
  than managed free tiers for no current benefit; still viable later via the same Docker artifacts.
- **SPA on Cloudflare Pages + an `/api` proxy.** Rejected for alpha — single origin via Pages needs a
  proxy in front (a Pages Function or a custom-domain rule) at *every* tier, carrying a two-host split
  and a proxy hop permanently for marginal CDN gain on a small hashed bundle. Serving the SPA from the
  Go app is one origin everywhere, and Cloudflare-in-front recovers the CDN at demo/production without
  the split. (This reverses the initial Pages choice once single-origin's proxy cost became concrete —
  preview should rehearse the production shape, and production wants one app behind a CDN, not a
  permanent two-host split.)

## Consequences

- Prerequisites: a multi-stage **Dockerfile** (SPA + Go binary), a **`fly.toml`** per app with the
  `goose up` `release_command` and `WEB_DIR` for the embedded SPA, a Neon project + per-env branches,
  a verified Resend domain, a per-env Google OAuth client, and GitHub Environments + secrets (only
  `FLY_API_TOKEN` per env — backend runtime secrets live on Fly via `fly secrets`). The workflow YAML
  is the small part; the provisioning is the real work.
- `dev` is reachable only while the laptop and foreground `cloudflared` are running — by design.
- Running cost at alpha ≈ **$0–2/mo** (Fly scale-to-zero; Neon and Resend free; no Cloudflare needed
  for `preview`).
- Adding `demo` then `production` later is another Fly app + Neon branch + the already-wired tag
  pattern and GitHub Environment.
- Vendor-shift risk (ADR-0013's stated concern) stays bounded: no vendor SDK lock-in, `pg_dump`
  portability holds, and the image runs anywhere — so leaving Fly/Neon/Cloudflare is a config
  change, not a migration.
- A future ADR records `production` specifics (backups, observability, custom domain, rollout
  policy) when it is stood up.
