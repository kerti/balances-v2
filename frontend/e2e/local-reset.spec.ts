import { test, expect } from "@playwright/test";

// Drives the emailed password-reset UI end-to-end (ADR-0039, #282): the sign-in
// screen's "Forgot password?" link, the request form's generic confirmation
// (which never reveals whether the email exists), and the invalid-link notice on
// the set screen. The e2e backend runs with AUTH_LOCAL_ENABLED + EMAIL_ENABLED
// (the default), so the methods endpoint advertises reset and the link renders.
//
// The full request → emailed-token → set → login round-trip is verified by the
// backend handler tests against a real DB (reset_test.go, INV-AUTH-19): the reset
// token is delivered ONLY by email, and the e2e harness has no mail capture, so
// the token can't be retrieved here. This spec owns the SPA wiring.
//
// Start unauthenticated by overriding the project's injected storageState.
test.use({ storageState: { cookies: [], origins: [] } });

test(
  "forgot-password link leads to the request form and a generic confirmation",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.goto("/");

    // The local form is shown (local auth on) with the reset affordance (email on).
    await expect(page.getByTestId("local-auth-form")).toBeVisible();
    await page.getByTestId("local-forgot-password").click();

    // On the request form: submitting any email lands on the same generic
    // confirmation — no enumeration, so the UI can't reveal account existence.
    await expect(page.getByTestId("reset-request-form")).toBeVisible();
    await page
      .getByTestId("reset-request-email")
      .fill(`nobody-${Date.now()}@example.test`);
    await page.getByTestId("reset-request-submit").click();
    await expect(page.getByTestId("reset-request-sent")).toBeVisible();
  },
);

test("an invalid reset link shows the request-a-new-one notice", async ({
  page,
}) => {
  // A bogus token resolves to the generic 409, so the set screen replaces the
  // form with the invalid-link notice rather than a password field.
  await page.goto("/reset?token=this-token-does-not-exist");
  await expect(page.getByTestId("reset-invalid")).toBeVisible();
  await expect(page.getByTestId("reset-set-form")).toHaveCount(0);
});
