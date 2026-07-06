import { test, expect } from "@playwright/test";

// Drives the REAL OAuth login flow end-to-end: Sign-in link -> /auth/google/start
// -> mock OIDC authorize -> callback -> minted session -> back to the app. This is
// the one path session-injection (auth.spec.ts) cannot cover. It runs against the
// local mock-oidc provider that `make e2e` starts (ADR-0024 option B), so nothing
// here contacts accounts.google.com. The mock issues an id_token for sub=e2e-alice,
// which matches the seeded Alice, so we land as the existing fixture user (a new
// random session is minted; the seeded session is untouched).
//
// Override the project's injected storageState with an empty one so the context
// starts unauthenticated and the sign-in screen renders first.
test.use({ storageState: { cookies: [], origins: [] } });

// covers: INV-JOURNEYS-01
test(
  "signs in via the mock OIDC provider and lands as Alice",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.goto("/");

    const signIn = page.getByTestId("signin-google");
    await expect(signIn).toBeVisible();

    // Full-page navigation through the redirect chain; auto-waiting assertions
    // below cover the async settle back on the e2e frontend origin.
    await signIn.click();

    // Authenticated AppShell renders as the seeded Alice.
    await expect(page.getByText("Alice", { exact: true })).toBeVisible();
    await expect(page.getByText("alice@example.com")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
  },
);

// covers: INV-AUTH-09
// The pre-auth language picker is display-only: choosing a language switches the
// unauthenticated UI and threads the choice onto the start link as ?lng= (which
// the backend turns into the oauth_locale seed hint, ADR-0035). It never PATCHes
// — verified here only as far as the link/copy reacting; the server-side seed is
// covered by the Go suite.
test(
  "pre-auth language picker threads the locale onto the start link",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.goto("/");

    const picker = page.getByTestId("signin-language-select");
    await expect(picker).toBeVisible();
    const signIn = page.getByTestId("signin-google");

    await picker.selectOption("id-ID");
    await expect(signIn).toHaveAttribute("href", /[?&]lng=id-ID/);
    // The visible UI switches to the chosen language too.
    await expect(signIn).toHaveText("Lanjutkan dengan Google");

    await picker.selectOption("en-GB");
    await expect(signIn).toHaveAttribute("href", /[?&]lng=en-GB/);
    await expect(signIn).toHaveText("Continue with Google");
  },
);
