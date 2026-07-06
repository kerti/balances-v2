import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

// The fixed session cookie value seeded by `balances seed-e2e` (ADR-0024).
// Injecting it as storageState authenticates every test as Alice without
// driving Google OAuth. It is a constant, not a random token, because it only
// ever exists in the balances_e2e database.
const E2E_SESSION_ID = "e2e-session-alice";

const authFile = path.resolve(path.dirname(fileURLToPath(import.meta.url)), ".auth/state.json");

// globalSetup writes the storageState file Playwright loads for every context.
// It does not seed the database — `make e2e` does that synchronously before
// Playwright starts, so the backend's auto-migrate never races the seed.
export default async function globalSetup() {
  const state = {
    cookies: [
      {
        name: "session",
        value: E2E_SESSION_ID,
        domain: "localhost",
        path: "/",
        // Far future; the seeded session row's own expiry is the real gate.
        expires: Math.floor(Date.UTC(2100, 0, 1) / 1000),
        httpOnly: true,
        secure: false,
        sameSite: "Lax" as const,
      },
    ],
    origins: [
      {
        // Pin the UI to en-GB so specs that assert English copy aren't
        // affected by the runner's navigator.language. The seeded user row
        // also carries locale='en-GB' (cmd/balances seed-e2e); pre-priming
        // localStorage here additionally skips the AppShell's first-login
        // navigator reconciliation. To exercise the ID UI in a spec, switch
        // via the Settings dropdown rather than mutating this seed.
        //
        // balances.theme is pinned to 'dark' for the same reason: Playwright
        // emulates prefers-color-scheme: light by default, so without this the
        // theme reconcile (useThemeReconcile) would flip every spec's first
        // login to light. The seeded row defaults to dark (users.theme, migration
        // 00024); pinning localStorage to match skips the reconcile. To exercise
        // the light UI, switch via the Settings dropdown (see theme.spec.ts).
        origin: "http://localhost:5273",
        localStorage: [
          { name: "balances.locale", value: "en-GB" },
          { name: "balances.theme", value: "dark" },
        ],
      },
    ],
  };

  await mkdir(path.dirname(authFile), { recursive: true });
  await writeFile(authFile, JSON.stringify(state, null, 2));
}
