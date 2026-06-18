# Self-hostable docker-compose stack

The operator-facing deployment artifact (#116) is a single **pull-based** `docker-compose.yml` at the
repo root: the published image (`ghcr.io/kerti/balances:<tag>`) plus PostgreSQL, with mail and a
reverse proxy as optional profiles. This is the concrete form of the upgrade contract [[adr-0033]]
made the versioned surface — a stranger's `docker compose pull && up -d` — and it mirrors the
single-origin shape [[adr-0030]] runs on Fly, repackaged for a machine we don't own. First shipped at
**`v1.0.0`** (the self-host stack is a `1.0.0` blocker per [[adr-0033]]).

## Why now

[[adr-0033]] settled that the version string is the operator's upgrade contract *because* Balances is
self-hostable, but deferred the artifact that makes the contract real. [[adr-0030]] designed our Fly
hosting, not a self-hoster's. The existing repo-root `docker-compose.yml` was dev scaffolding
(postgres + mailpit, no app service). This ADR designs what the operator actually runs.

## The decision

### One pull-based operator file; dev scaffolding steps aside

- The repo-root **`docker-compose.yml` is the operator artifact**: `app` (a *published* image, never
  `build:`), `postgres` (named volume), and profiled `mailpit` / `caddy`. A bare `docker compose up`
  does the right thing with no flags.
- Dev's postgres+mailpit move to **`docker-compose.dev.yml`**; `make up` points there. Dev complexity
  does not leak onto the operator's face.
- **Releases publish the image to GHCR** (`ghcr.io/kerti/balances:<tag>`). Today we only `flyctl
  deploy` (builds on Fly, pushes nothing public); the upgrade contract's `docker compose pull` is only
  real if a versioned image exists in a public registry. This is a new release-pipeline step.

### Migrations run as a discrete one-shot service, not in the app entrypoint

Fly runs `release_command = "migrate up"` in a separate one-off VM so a failed migration blocks the
rollout. Compose has no `release_command`, so a one-shot **`migrate` service** reproduces it: the same
image with `command: ["migrate","up"]`, `depends_on: postgres: condition: service_healthy`. The `app`
service then `depends_on: migrate: condition: service_completed_successfully`. A migration failure
surfaces as a clearly failed `migrate` container and the app never starts half-migrated — the issue's
"block startup rather than boot half-migrated." Keeps the app service single-purpose (CMD `serve`),
matching the Dockerfile and Fly. Rejected: `migrate up && serve` in the entrypoint — a failure becomes
an app crash-loop (harder for an operator to diagnose) and a wrapper script is impossible on the
distroless base (no shell).

### One `APP_URL` collapses the single-origin URL knobs

Single-origin means `FRONTEND_URL`, `BACKEND_URL`, and the OAuth callback all resolve to the
operator's one origin. Asking an operator to set three vars in agreement — and hand-derive the
`/api/auth/google/callback` suffix — is the OAuth-redirect footgun that bit the alpha. So **`APP_URL`**
defaults `FrontendURL`/`BackendURL` and derives `OAuthRedirectURL = APP_URL +
/api/auth/google/callback`. The three original vars remain (overridable) for the split-origin dev path
and Fly. `COOKIE_SECURE` keeps its `false` default so a bare-`localhost` http trial works out of the
box; the TLS topologies below flip it to `true`.

### Three supported topologies; TLS never terminates in the app

Balances always speaks plain http behind whatever terminates TLS — as it already does behind Fly. The
`app` port is **published by default** (`${APP_PORT:-8080}:8080`) so an external proxy can reach it.

| Topology | Profile | TLS terminated by | Operator sets |
|---|---|---|---|
| Localhost trial | none | nobody (http) | `APP_URL=http://localhost:8080`, `COOKIE_SECURE=false` |
| Bring-your-own proxy | none | their existing reverse proxy | `APP_URL=https://<domain>`, `COOKIE_SECURE=true`, point their RP at the published port |
| Bundled turnkey | `proxy` | **Caddy** (automatic Let's Encrypt) | `APP_URL=https://<domain>`, `COOKIE_SECURE=true`, domain in the `Caddyfile` |

**Caddy** is the bundled proxy (a three-line `Caddyfile`, automatic HTTPS) over Traefik, whose
label-driven power is overhead for one app + one db. The bring-your-own-proxy path (homelab / Proxmox
with the operator's own certs) is co-equal, not an afterthought — it is simply the bundled profile left
off. Off by default because localhost trials need no TLS.

### Mail is optional via a single `EMAIL_ENABLED` flag

A self-hoster who wants no mail dependency sets `EMAIL_ENABLED=false`: the mail sender becomes a no-op
and SMTP config is unneeded. The only mail with a hard dependency is **invitations** — but the create
endpoint already returns the `accept_url` and mail is already best-effort, so the fallback is a
front-end **"copy invite link"** button. Welcome and backup/restore-complete mails silently no-op.
This is the first concrete target of the feature-flag framework (#223), built as **one env flag, not a
framework** — the framework earns its keep at the second flag (alt-auth), not the first.

### Auth stays Google-only; the non-Google path is deferred to a flag

Self-host v1.0.0 ships Google OAuth only, per [[adr-0017]]. The whole identity and portability model is
keyed on the Google `sub` (immutable identity key, backup/restore re-link, invitation email-match), so
a local-password or generic-OIDC path is not a config toggle — it reopens that model. It is deferred to
#223's "next possible target" as an additive flagged feature, exactly the additive layer [[adr-0017]]
anticipated. The one genuinely manual operator step — registering a Google OAuth client — is answered
with a thorough walkthrough in `SELF-HOSTING.md`, not by removing the dependency.

## Consequences

- A new **GHCR publish-on-release** step is required before the upgrade contract is real; it is part of
  the M7 self-host deliverable.
- A small set of additive code changes land alongside the stack: `APP_URL` in `config.Load`, the
  `EMAIL_ENABLED` flag, and a "copy invite link" affordance on the invitation flow.
- `SELF-HOSTING.md` becomes a maintained artifact: three-topology quickstart, the OAuth-client
  walkthrough, the upgrade contract ([[adr-0033]]), PostgreSQL-volume `pg_dump`/`pg_restore` guidance,
  and troubleshooting.
- Out of scope (per #116): Kubernetes/Helm, multi-tenant SaaS infra, non-Docker install paths.
