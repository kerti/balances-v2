# Self-hosting Balances

Balances runs as a small Docker Compose stack: the published application image, a PostgreSQL
database, and — optionally — a reverse proxy for HTTPS. There is nothing to build. You pull a
released image and bring it up.

This guide is the operator's entry point. It covers three ways to run the stack, how users sign in
(Google, local email+password, or both), how upgrades work, how to back up your database, and what to
do when something goes wrong.

> **Two ways to sign in.** By default users sign in with a Google account, which means creating a
> Google OAuth client (the one manual step) — [Google OAuth client](#google-oauth-client). If you'd
> rather avoid Google entirely, turn on **local email + password** accounts and skip the OAuth client
> altogether — [Local accounts (no Google)](#local-accounts-no-google). You can run either or both.

## Contents

- [What you need](#what-you-need)
- [Quickstart: three topologies](#quickstart-three-topologies)
  - [1. Localhost trial (http, no proxy)](#1-localhost-trial-http-no-proxy)
  - [2. Bring your own proxy](#2-bring-your-own-proxy)
  - [3. Bundled turnkey HTTPS (Caddy)](#3-bundled-turnkey-https-caddy)
- [Google OAuth client](#google-oauth-client)
- [Local accounts (no Google)](#local-accounts-no-google)
- [Email (optional)](#email-optional)
- [The upgrade contract](#the-upgrade-contract)
- [Backup and restore](#backup-and-restore)
- [Troubleshooting](#troubleshooting)
- [Reference: configuration](#reference-configuration)

## What you need

- A machine with **Docker** and the **Docker Compose v2** plugin (`docker compose`, not the old
  `docker-compose`). A 1 GB / 1 vCPU VM is comfortable.
- The two stack files from this repository: **`docker-compose.yml`** and **`Caddyfile`**, plus
  **`.env.example`**. Download them into an empty directory:

  ```sh
  mkdir balances && cd balances
  base=https://raw.githubusercontent.com/kerti/balances-v2/main
  curl -fsSLO "$base/docker-compose.yml"
  curl -fsSLO "$base/Caddyfile"
  curl -fsSL  "$base/.env.example" -o .env
  ```

  Use a directory **outside any clone of the source repository**. The stack pins
  `COMPOSE_PROJECT_NAME=balances` in `.env`, so it owns an isolated `balances_*`
  namespace regardless of where you run it — but running it from inside the
  checkout (which also carries a `docker-compose.dev.yml` for development) is
  asking for confusion. A dedicated directory keeps the operator stack cleanly
  separate.

- A pinned release tag. The image is published to the GitHub Container Registry as
  `ghcr.io/kerti/balances:<tag>` (e.g. `v1.0.0`). **No `latest` tag is published** — you always pin a
  real version so upgrades are deliberate. Browse releases at
  <https://github.com/kerti/balances-v2/releases>.

Everything else (PostgreSQL, migrations) is in the compose file. The application always speaks plain
HTTP; TLS is terminated in front of it (by your proxy or the bundled Caddy), exactly as it is in the
hosted deployment.

## Quickstart: three topologies

All three start from the same `.env`. Open it and set, at minimum:

```sh
BALANCES_TAG=v1.0.0          # the release you want to run
POSTGRES_PASSWORD=...        # change this for anything past a localhost trial
GOOGLE_CLIENT_ID=...         # see "Google OAuth client" below
GOOGLE_CLIENT_SECRET=...
```

The only knob that differs between topologies is **how Balances is reached** — `APP_URL`,
`COOKIE_SECURE`, and whether a proxy is in front. `APP_URL` is the single source of truth: it derives
the frontend URL, the backend URL, **and** the OAuth callback. Set it correctly and there is nothing
else to keep in sync.

### 1. Localhost trial (http, no proxy)

The fastest way to kick the tyres on the machine itself. No TLS, no domain.

```ini
APP_URL=http://localhost:8080
COOKIE_SECURE=false
```

```sh
docker compose up -d
```

Compose starts PostgreSQL, waits for it to be healthy, runs the one-shot `migrate` service to apply
all migrations, then starts the app. Open <http://localhost:8080>.

For the OAuth client, the redirect URI is `http://localhost:8080/api/auth/google/callback` (Google
permits `http` **only** for `localhost`).

> A localhost trial is not reachable from other machines and has no HTTPS. Do not use it for real
> data shared with your household — use one of the two topologies below.

### 2. Bring your own proxy

You already run a reverse proxy (Nginx, Traefik, Caddy, a homelab/Proxmox setup with your own
certificates). Balances stays plain HTTP on its published port; your proxy terminates TLS and forwards
to it. This path is fully supported — it is simply the bundled proxy profile left off.

```ini
APP_URL=https://balances.example.com
COOKIE_SECURE=true
```

```sh
docker compose up -d
```

The app is published on `APP_PORT` (default `8080`). Point your proxy at `http://<host>:8080` and have
it serve `https://balances.example.com`. Forward the `Host`, `X-Forwarded-Proto`, and
`X-Forwarded-For` headers as usual.

`COOKIE_SECURE=true` is required: the session cookie is then marked `Secure` and is only sent over
HTTPS, which your proxy now provides. (Leaving it `false` behind HTTPS is harmless; setting it `true`
without HTTPS will lock you out — see [Troubleshooting](#troubleshooting).)

The OAuth redirect URI is `https://balances.example.com/api/auth/google/callback`.

### 3. Bundled turnkey HTTPS (Caddy)

No existing proxy? The stack ships an optional [Caddy](https://caddyserver.com) profile that obtains
and auto-renews a Let's Encrypt certificate for your domain. You get HTTPS with no certificate
handling of your own.

Prerequisites:

- A domain name whose DNS **A/AAAA record points at this machine's public IP**.
- **Ports 80 and 443 reachable from the internet** — Let's Encrypt validates over them.

```ini
APP_URL=https://balances.example.com
COOKIE_SECURE=true
CADDY_DOMAIN=balances.example.com
```

```sh
docker compose --profile proxy up -d
```

`CADDY_DOMAIN` is read by the bundled `Caddyfile`; you do not edit the file. Caddy fetches a
certificate on first boot (a few seconds) and proxies to the app. Issued certificates persist in the
`caddy_data` volume, so restarts do not re-request them (which matters — Let's Encrypt rate-limits).

The OAuth redirect URI is `https://balances.example.com/api/auth/google/callback`.

## Google OAuth client

Balances signs users in with Google (see [ADR-0017](docs/adr/0017-google-oauth-server-side-sessions-and-email-token-invitations.md)). You create
one OAuth client in Google Cloud and paste its ID and secret into `.env`. This is the only step that
cannot be automated. It takes about five minutes and you never have to touch it again.

1. Go to the [Google Cloud Console](https://console.cloud.google.com/) and **create a project** (top
   bar → project picker → *New project*). Name it anything, e.g. `balances`.
2. **Configure the OAuth consent screen** — *APIs & Services → OAuth consent screen*.
   - User type: **External**.
   - App name, support email, developer contact: fill in your own.
   - Scopes: the defaults are enough (the app requests `openid`, `email`, `profile`). You do **not**
     need to add any.
   - Test users: while the consent screen is in *Testing* status, add every Google account that will
     sign in (yourself and your household members) under *Test users*. Without this, Google blocks
     sign-in with an "app not verified" / "access blocked" error. (Publishing the app removes the
     test-user requirement but is optional for a private household instance.)
3. **Create the OAuth client** — *APIs & Services → Credentials → Create credentials → OAuth client
   ID*.
   - Application type: **Web application**.
   - **Authorised redirect URI** — this is the value that must be exact. The formula is:

     ```
     <APP_URL>/api/auth/google/callback
     ```

     Substitute your `APP_URL` verbatim. Examples:

     | Topology | `APP_URL` | Authorised redirect URI |
     |---|---|---|
     | Localhost trial | `http://localhost:8080` | `http://localhost:8080/api/auth/google/callback` |
     | Proxy / Caddy | `https://balances.example.com` | `https://balances.example.com/api/auth/google/callback` |

     It must match scheme, host, port, and path **exactly** — no trailing slash, no `www` it doesn't
     have. A mismatch is the most common setup failure; see [Troubleshooting](#troubleshooting).
4. Copy the generated **Client ID** and **Client secret** into `.env`:

   ```ini
   GOOGLE_CLIENT_ID=....apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=...
   ```

5. Recreate the app so it picks up the new values: `docker compose up -d`.

The first person to sign in **founds** the household; everyone else joins by invitation from inside
the app.

## Local accounts (no Google)

If you'd rather not create a Google OAuth client, turn on **local email + password** accounts. Two
flags select which sign-in methods are live:

```env
AUTH_GOOGLE_ENABLED=false    # default true
AUTH_LOCAL_ENABLED=true      # default false
```

With Google off and local on — the recommended posture for a home server on a Raspberry Pi — the app
needs **no Google credentials** and makes **no outbound calls to Google** at all. Combined with
`EMAIL_ENABLED=false`, this is a fully self-contained instance with no third-party dependency: founder
registers and logs in locally, members join via the copy-invite-link panel, and there is nothing
external to configure. You can also leave **both** providers on (`AUTH_GOOGLE_ENABLED=true` and
`AUTH_LOCAL_ENABLED=true`) — the sign-in screen then shows the Google button **and** the email/password
form. The server refuses to start if you disable both (there would be no way to sign in).

Passwords are stored hashed with Argon2id (never in plaintext, never in a backup file) and must be at
least 10 characters and not a commonly-breached password. Login is rate-limited to slow down guessing,
but never hard-locks an account — you can't accidentally lock yourself or a housemate out.

**Inviting a member who has no Google account.** Send them their **invite link** (the "copy invite
link" button, or the invitation email when mail is on). Following the link lands them on a *set your
password* page bound to the email you invited — they pick a password and are signed straight in, no
Google needed. The link is **single-use and time-limited**: once it has been used (or it expires) it
can't create a second account, and a forwarded-after-use link is inert. If a link is spent or stale,
just send a fresh invitation.

> ### ⚠️ Found the household before exposing the instance
>
> With local accounts, **the first person to register founds the household** — and on a fresh
> instance there is no way to verify that whoever reaches the sign-up page first actually owns the
> email address they type. This is a deliberate trade for zero-dependency bring-up, and it is a
> **first-run window only**: once the household exists, every further local account is invite-only.
>
> The safe sequence is: **stand the instance up privately (LAN / VPN / Tailscale), register the
> founder account immediately, and only then expose it** to any wider network. Do **not** put a
> freshly-deployed, unfounded local-auth instance on the public internet and walk away — register
> first. (Google-only deployments are not affected: Google verifies the email.)
>
> After a backup **restore**, local members land **dormant** — their identity and data are present,
> but their password did not travel in the backup file (secrets never do). Reactivate each member
> from the app (founder-assisted) or with the `reset-password` operator command. This is expected and
> is the price of keeping password hashes out of the portable backup.

## Email (optional)

Email is **off by default** (`EMAIL_ENABLED=false`). The stack runs perfectly with no mail server:

- **Invitations** show a **"copy invite link"** button — copy the link and send it however you like.
- **Welcome** and **backup/restore-complete** notifications silently do nothing.

To turn email on, set `EMAIL_ENABLED=true` and either point the `SMTP_*` variables at your own relay
or bring up the bundled [Mailpit](https://mailpit.axllent.org/) sink for testing:

```sh
docker compose --profile mail up -d        # adds the mailpit service
```

```ini
EMAIL_ENABLED=true
SMTP_HOST=mailpit
SMTP_PORT=1025
EMAIL_FROM_ADDRESS=noreply@balances.local
```

Mailpit's web UI (captured mail) is at `http://<host>:8025`. For real outbound mail, use your own
provider's SMTP credentials instead and set `EMAIL_FROM_ADDRESS` to an address that provider is
authorised to send from.

## The upgrade contract

The version number tells you what an upgrade will cost. This is a deliberate contract — see
[ADR-0033](docs/adr/0033-versioning-the-upgrade-contract-and-migration-immutability.md). Releases follow
[SemVer](https://semver.org/) (`MAJOR.MINOR.PATCH`):

| Release | Meaning | What you do |
|---|---|---|
| **patch** (`v1.0.0` → `v1.0.1`) | bug/hotfix, no migration | drop-in: bump `BALANCES_TAG`, `pull && up -d` |
| **minor** (`v1.0.0` → `v1.1.0`) | additive feature + additive migration | drop-in: `pull && up -d`; the migration runs automatically on boot |
| **major** (`v1.0.0` → `v2.0.0`) | breaking but **your data survives** — a destructive/irreversible migration, a required config change, or a dropped upgrade path | **read the release notes first**; expect manual steps |
| **new repo** | data **cannot** forward-migrate | a fresh install of the next generation |

For a routine patch or minor upgrade:

```sh
# 1. Back up first (see below) — always, even for a patch.
# 2. Edit .env: BALANCES_TAG=v1.1.0
docker compose pull
docker compose up -d
```

`pull` fetches the new image; `up -d` runs the one-shot `migrate` service (applying any new
migrations) and then restarts the app on the new version. The database volume is untouched by the
upgrade itself.

> Always read the [release notes](https://github.com/kerti/balances-v2/releases) for the version
> you're moving to. A **major** bump means there is something there you need to act on.

## Backup and restore

There are **two** distinct kinds of backup; don't confuse them:

- **In-app Household Backup/Restore** — a feature inside Balances (Settings) that exports your
  household's data as a portable file. Good for moving a household between instances. This is a
  product feature, not an ops task.
- **PostgreSQL volume backup** (below) — a full database dump at the infrastructure level. This is
  your disaster-recovery safety net and what you take before every upgrade.

The database lives in the named Docker volume `postgres_data`. Dump and restore it through the running
`postgres` service so you never have to hand-write connection details.

**Back up** (writes a compressed custom-format dump to the host):

```sh
docker compose exec -T postgres \
  pg_dump -U "${POSTGRES_USER:-balances}" -d "${POSTGRES_DB:-balances}" -Fc \
  > balances-$(date +%F).dump
```

**Restore** into a fresh, empty database (this **replaces** current data — stop the app first):

```sh
docker compose stop app
docker compose exec -T postgres \
  pg_restore -U "${POSTGRES_USER:-balances}" -d "${POSTGRES_DB:-balances}" \
  --clean --if-exists < balances-2026-06-18.dump
docker compose up -d app
```

Notes:

- `-Fc` (custom format) pairs with `pg_restore` and supports `--clean --if-exists`, which drops and
  recreates objects so a restore is idempotent. For a plain SQL dump instead, drop `-Fc` from
  `pg_dump` and restore with `psql ... < file.sql`.
- The `-T` flag disables TTY allocation so the redirected input/output streams work over
  `docker compose exec`.
- Keep dumps off the server (copy them elsewhere). A backup on the same disk that dies with the disk
  is not a backup.
- Take a dump **before every upgrade**, including patches.

## Troubleshooting

**The `migrate` container exited / the app never starts.**
A migration failed, and by design the app refuses to start half-migrated. Inspect it:

```sh
docker compose logs migrate
```

The `migrate` service runs `migrate up` once and exits `0` on success; the `app` service waits for
that success. A non-zero exit leaves a clearly failed `migrate` container and no app. Fix the cause
(usually a bad `DATABASE_URL`-derived password or an interrupted prior upgrade), then `docker compose
up -d` to retry. The database is never left half-migrated.

**I can sign in but get bounced straight back to the login page (or the cookie won't stick).**
Almost always `COOKIE_SECURE=true` while the site is served over plain HTTP. A `Secure` cookie is
dropped by the browser on non-HTTPS connections, so the session never persists. Either put HTTPS in
front (topologies 2 and 3) or set `COOKIE_SECURE=false` for an http localhost trial. Recreate the app
after changing it: `docker compose up -d`.

**Google says `redirect_uri_mismatch` (or "Error 400: redirect_uri_mismatch").**
The Authorised redirect URI on your OAuth client does not byte-for-byte match what the app sends. The
app always sends `<APP_URL>/api/auth/google/callback`. Check, in the Google Cloud Console:

- scheme matches (`http` vs `https`),
- host matches exactly (no stray `www`, no IP-vs-hostname mismatch),
- port matches (`:8080` for the localhost trial, none for a standard `https` domain),
- path is exactly `/api/auth/google/callback` with **no trailing slash**.

Update `APP_URL` and the redirect URI together so they agree, then `docker compose up -d`.

**"Access blocked: this app's request is invalid" / "has not completed verification".**
Your Google account isn't on the consent screen's **Test users** list while the app is in *Testing*
status. Add it (or publish the app). See [Google OAuth client](#google-oauth-client) step 2.

**Inviting someone does nothing / no email arrives.**
Expected with `EMAIL_ENABLED=false` — use the **copy invite link** button on the invitation and send
the link yourself. To send real mail, see [Email](#email-optional).

**Caddy won't get a certificate.**
Confirm the domain's DNS points at this machine and that ports 80 and 443 are open to the internet
(cloud firewall **and** host firewall). Watch `docker compose logs caddy` during first boot. Issued
certs are cached in `caddy_data`; repeated failures can hit Let's Encrypt rate limits, so fix DNS/
ports before retrying in a loop.

## Reference: configuration

Every variable lives in `.env`; `.env.example` is the annotated source of truth. The ones you'll
actually touch:

| Variable | Default | Purpose |
|---|---|---|
| `BALANCES_TAG` | `v1.0.0` | Released image tag to run. Pin a real version; bump to upgrade. |
| `APP_PORT` | `8080` | Host port the single origin (SPA + `/api`) is published on. |
| `APP_URL` | `http://localhost:8080` | The one origin the app is reached on. Derives frontend/backend URLs and the OAuth callback. |
| `COOKIE_SECURE` | `false` | `true` once HTTPS is in front; `false` only for an http localhost trial. |
| `CADDY_DOMAIN` | _(empty)_ | Domain for the bundled Caddy proxy profile. Leave empty unless using `--profile proxy`. |
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | `balances` | Bundled database credentials. **Change the password** beyond a trial. `DATABASE_URL` is assembled from these — you do not set it. |
| `AUTH_GOOGLE_ENABLED` | `true` | Google sign-in. Set `false` to run without a Google OAuth client. |
| `AUTH_LOCAL_ENABLED` | `false` | `true` to enable local email + password accounts. The server refuses to start if both providers are disabled. |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | _(empty)_ | Your Google OAuth client. Required when `AUTH_GOOGLE_ENABLED=true`. |
| `EMAIL_ENABLED` | `false` | `true` to send mail (then set `SMTP_*`). Off = copy-invite-link fallback. |
| `SESSION_TTL` | `720h` | How long a sign-in lasts (default 30 days). |
| `LOG_FORMAT` | `json` | `json` or `text` application logs. |

Out of scope for self-hosting (per [issue #116](https://github.com/kerti/balances-v2/issues/116)):
Kubernetes/Helm, multi-tenant SaaS infrastructure, and non-Docker install paths. The supported
artifact is this Compose stack.
