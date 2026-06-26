import { defineConfig, devices } from '@playwright/test'

// E2E against a dedicated balances_e2e backend (port 8099) + a dedicated vite
// instance (port 5273) so the developer's 8080/5173 dev servers are never
// touched. Auth is injected as a session cookie via storageState (global
// setup) rather than driving Google OAuth — see ADR-0024. The DB is seeded by
// `make e2e` before Playwright launches, so the backend's auto-migrate is a
// no-op and there is no migration race. Entry point is `make e2e`.
const E2E_BACKEND_PORT = 8099
const E2E_FRONTEND_PORT = 5273
// The fake OIDC provider (cmd/balances mock-oidc) that `make e2e` launches
// before Playwright boots the backend. Defaults here mirror mock-oidc's own
// defaults; the backend discovers it at boot via OIDC_ISSUER_URL. See ADR-0024.
const E2E_OIDC_ISSUER = 'http://localhost:8090'
const E2E_OIDC_CLIENT_ID = 'e2e-client'
const E2E_OIDC_CLIENT_SECRET = 'e2e-secret'

export default defineConfig({
  testDir: './e2e',
  // Single household, single shared DB: tests must not run concurrently.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  globalSetup: './e2e/global-setup.ts',
  use: {
    baseURL: `http://localhost:${E2E_FRONTEND_PORT}`,
    storageState: 'e2e/.auth/state.json',
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
  webServer: [
    {
      command: 'go run ./cmd/balances serve',
      cwd: '../backend',
      env: {
        PORT: String(E2E_BACKEND_PORT),
        // Set by `make e2e`; the seed step has already migrated this DB.
        DATABASE_URL: process.env.E2E_DATABASE_URL ?? '',
        // Point auth at the local mock-oidc instead of accounts.google.com, so
        // boot-time OIDC discovery and the login flow stay offline. These keys
        // override any .env values (Playwright merges process.env then this).
        OIDC_ISSUER_URL: E2E_OIDC_ISSUER,
        GOOGLE_CLIENT_ID: E2E_OIDC_CLIENT_ID,
        GOOGLE_CLIENT_SECRET: E2E_OIDC_CLIENT_SECRET,
        // Callback comes back to the backend directly; the backend then sets the
        // session cookie (host-scoped 'localhost', shared across ports) and
        // redirects to the e2e frontend — mirroring the real dev wiring.
        OAUTH_REDIRECT_URL: `http://localhost:${E2E_BACKEND_PORT}/api/auth/google/callback`,
        FRONTEND_URL: `http://localhost:${E2E_FRONTEND_PORT}`,
        // Enable local password auth alongside Google (ADR-0039) so the
        // local-auth @smoke spec can register a founder and sign back in.
        AUTH_LOCAL_ENABLED: 'true',
      },
      url: `http://localhost:${E2E_BACKEND_PORT}/healthz`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
    {
      command: `npm run dev -- --port ${E2E_FRONTEND_PORT} --strictPort`,
      env: { API_PROXY_TARGET: `http://localhost:${E2E_BACKEND_PORT}` },
      url: `http://localhost:${E2E_FRONTEND_PORT}`,
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
    },
  ],
})
