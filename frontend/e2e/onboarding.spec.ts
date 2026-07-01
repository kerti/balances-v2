import { test, expect } from "@playwright/test";

// Drives a BRAND-NEW Google identity through the ADR-0038 onboarding gate in the
// browser (#274). The default mock-OIDC flow always issues the seeded Alice, so
// login.spec.ts can only ever exercise the existing-user branch — it can never
// reach the gate, which fires only for a google_sub with no matching user. Here
// we set the `mock_oidc_sub` cookie on the mock-oidc origin (:8090), which the
// browser sends on the top-level redirect to /authorize; mock-oidc then mints an
// id_token for that fresh sub (mockoidc.go). GetUserByGoogleSub misses, the
// callback records a handshake and bounces to /onboarding, and the founder choice
// bootstraps the Household + session. The Go suite owns the gate's server halves
// (INV-AUTH-05 handshake/redirect, INV-AUTH-13 founder commit); this pins the
// browser loop that stitches them — the sibling of INV-JOURNEYS-01's existing-user
// landing. Nothing here contacts accounts.google.com (ADR-0024 option B).
//
// Override the project's injected storageState with an empty one so the context
// starts unauthenticated and the sign-in screen renders first.
test.use({ storageState: { cookies: [], origins: [] } });

// The mock-oidc origin the e2e backend discovers via OIDC_ISSUER_URL
// (playwright.config.ts) — where the sub-selecting cookie must live.
const MOCK_OIDC_ORIGIN = "http://localhost:8090";

// covers: INV-JOURNEYS-04
test(
  "a first-time Google identity onboards as a founder through the gate",
  { tag: "@smoke" },
  async ({ page, context }) => {
    // Unique per run (and per retry) so every attempt is a genuinely unseeded
    // google_sub — the gate only fires when GetUserByGoogleSub misses, so a reused
    // sub from a prior attempt would take the existing-user branch and skip it.
    const sub = `e2e-founder-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
    // mockoidc.go derives the email from the sub; assert on it to prove we landed
    // as the new identity, not the seeded Alice.
    const email = `${sub}@e2e.example.com`;

    // Select the fresh identity: the browser carries this to mock-oidc's /authorize
    // on the top-level OAuth redirect.
    await context.addCookies([
      { name: "mock_oidc_sub", value: sub, url: MOCK_OIDC_ORIGIN },
    ]);

    await page.goto("/");

    const signIn = page.getByTestId("signin-google");
    await expect(signIn).toBeVisible();
    await signIn.click();

    // Brand-new google_sub → no existing user → the onboarding gate renders instead
    // of the app shell (the founder form shows directly when there are no invites).
    await expect(page.getByTestId("onboarding-card")).toBeVisible();

    // Name the household and commit the founder choice.
    await page
      .getByTestId("onboarding-household-name")
      .fill("Newcomer Household");
    await page.getByTestId("onboarding-found-submit").click();

    // The commit set the real session cookie; the app flips to the authed shell as
    // the brand-new founder (display name from the id_token, unique email).
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
    await expect(page.getByText("E2E Newcomer", { exact: true })).toBeVisible();
    await expect(page.getByText(email)).toBeVisible();
  },
);
