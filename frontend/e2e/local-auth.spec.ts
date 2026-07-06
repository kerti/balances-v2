import { test, expect } from "@playwright/test";

// Drives the local email+password identity provider end-to-end (ADR-0039, #280):
// register a founder through the onboarding gate, land in the authenticated app,
// sign out, and sign back in with the same credentials. The e2e backend runs with
// AUTH_LOCAL_ENABLED=true (Google also on — a both-providers deployment), so the
// sign-in screen renders the local form beside the Google button.
//
// Start unauthenticated by overriding the project's injected storageState.
test.use({ storageState: { cookies: [], origins: [] } });

// covers: INV-AUTH-15
test(
  "registers a local founder through the gate, then signs back in",
  { tag: "@smoke" },
  async ({ page }) => {
    // Unique per run so reruns against a reused server don't collide on EMAIL_TAKEN.
    const email = `founder-${Date.now()}@example.test`;
    const password = "a-decent-founder-passphrase";
    const displayName = "E2E Local Founder";

    await page.goto("/");

    // Switch the local form to register and fill it in.
    await page.getByTestId("local-mode-register").click();
    await page.getByTestId("local-display-name").fill(displayName);
    await page.getByTestId("local-email").fill(email);
    await page.getByTestId("local-password").fill(password);
    await page.getByTestId("local-submit").click();

    // Register routes through the onboarding gate (no account yet) — commit the
    // founder choice (household name is pre-filled from the display name).
    await expect(page.getByTestId("onboarding-found-submit")).toBeVisible();
    await page.getByTestId("onboarding-found-submit").click();

    // Authenticated app shell renders as the new founder.
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
    await expect(page.getByText(displayName, { exact: true })).toBeVisible();

    // Sign out, then sign back in via the local form (default tab is "Sign in").
    await page.getByRole("button", { name: "Sign out" }).click();
    await expect(page.getByTestId("local-email")).toBeVisible();

    await page.getByTestId("local-email").fill(email);
    await page.getByTestId("local-password").fill(password);
    await page.getByTestId("local-submit").click();

    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
    await expect(page.getByText(displayName, { exact: true })).toBeVisible();
  },
);
