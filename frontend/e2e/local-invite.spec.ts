import { test, expect } from "@playwright/test";

// Drives the local-invite accept path end-to-end (ADR-0039, #281): a local
// founder invites a household member who has no Google account; the invitee
// follows the link, sets a password, and lands in the app — the account is
// created bound to the invited email with no second onboarding gate. The e2e
// backend runs with AUTH_LOCAL_ENABLED=true, so the invite link is the SPA
// /accept route rather than a Google /start URL.
//
// Start unauthenticated by overriding the project's injected storageState.
test.use({ storageState: { cookies: [], origins: [] } });

// covers: INV-AUTH-18
test(
  "invited local member sets a password and lands in the app",
  { tag: "@smoke" },
  async ({ page, browser }) => {
    const stamp = Date.now();
    const founderEmail = `inviter-${stamp}@example.test`;
    const founderPassword = "a-decent-founder-passphrase";
    const inviteeEmail = `invitee-${stamp}@example.test`;
    const inviteePassword = "a-decent-invitee-passphrase";

    // 1. Register a local founder through the onboarding gate.
    await page.goto("/");
    await page.getByTestId("local-mode-register").click();
    await page.getByTestId("local-display-name").fill("E2E Inviter");
    await page.getByTestId("local-email").fill(founderEmail);
    await page.getByTestId("local-password").fill(founderPassword);
    await page.getByTestId("local-submit").click();
    await page.getByTestId("onboarding-found-submit").click();
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();

    // 2. As the founder, mint an invitation. page.request shares the founder's
    //    session cookie, and the accept_url is the SPA /accept route (local on).
    const res = await page.request.post("/api/invitations", {
      data: { email: inviteeEmail },
    });
    expect(res.status()).toBe(201);
    const { accept_url: acceptURL } = (await res.json()) as {
      accept_url: string;
    };
    expect(acceptURL).toContain("/accept?token=");
    // Navigate relative so the test is independent of the configured FRONTEND_URL.
    const acceptPath = acceptURL.slice(acceptURL.indexOf("/accept"));

    // 3. The invitee opens the link in a FRESH context — no founder session, so
    //    App.tsx renders the accept screen rather than the authed app.
    const inviteeContext = await browser.newContext({
      storageState: { cookies: [], origins: [] },
    });
    const inviteePage = await inviteeContext.newPage();
    await inviteePage.goto(acceptPath);

    // The form is bound to the invited email and shown read-only.
    await expect(inviteePage.getByTestId("invite-accept-form")).toBeVisible();
    await expect(inviteePage.getByTestId("invite-email")).toHaveValue(
      inviteeEmail,
    );

    // 4. Set a password — the account is created and a session minted directly.
    await inviteePage.getByTestId("invite-password").fill(inviteePassword);
    await inviteePage.getByTestId("invite-submit").click();
    await expect(
      inviteePage.getByRole("button", { name: "Sign out" }),
    ).toBeVisible();

    // 5. The single-use link is now spent: reopening it shows the invalid notice.
    await inviteePage.context().clearCookies();
    await inviteePage.goto(acceptPath);
    await expect(inviteePage.getByTestId("invite-invalid")).toBeVisible();

    await inviteeContext.close();
  },
);
