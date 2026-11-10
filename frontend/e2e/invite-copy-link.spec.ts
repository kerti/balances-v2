import { test, expect } from "@playwright/test";

// Grant clipboard read+write so navigator.clipboard resolves in headless
// Chromium (localhost is a secure context; the permission is the missing piece).
test.use({ permissions: ["clipboard-read", "clipboard-write"] });

// covers: INV-NOTIFICATIONS-10
// covers: INV-NOTIFICATIONS-11
// covers: INV-AUTH-32
// The "copy invite link" affordance — the fallback that makes invitations work
// with EMAIL_ENABLED=false (ADR-0037), where the invite email never sends. The
// create endpoint returns the AcceptURL regardless, so the inviter copies the
// link and shares it by hand. Creates a real invitation through the UI as the
// seeded Alice, then exercises copy: the button flips to "Copied!" and the
// clipboard holds the accept URL — which is never rendered on screen (#847).
//
// Also pins the no-mail half of INV-NOTIFICATIONS-11: the e2e backend runs the
// NoopMailer (EMAIL_ENABLED=false), so Send succeeds and email_sent=true — the
// inviter must NOT be nagged with the "couldn't be sent" toast here; the
// copy-link button is the affordance. The email_sent=false toast itself can't be
// exercised against this real stack (NoopMailer never errors); the backend
// best-effort test guards that branch.
test("invite flow surfaces a copyable accept link", { tag: "@smoke" }, async ({ page }) => {
  await page.goto("/settings");

  const inviteEmail = `e2e-invitee-${Date.now()}@example.com`;
  await page.getByLabel("Email address").fill(inviteEmail);
  await page.getByRole("button", { name: "Send invitation" }).click();

  // Result block: who the invite went to, and a Copy button — no URL on screen.
  await expect(page.getByText(`Invite sent to ${inviteEmail}`)).toBeVisible();
  await expect(page.getByTestId("invite-result")).not.toContainText("/accept?token=");
  await expect(page.locator("body")).not.toContainText("token=");

  const copyButton = page.getByTestId("copy-invite-link");
  await expect(copyButton).toBeVisible();
  await copyButton.click();
  await expect(copyButton).toHaveText("Copied!");

  // The e2e backend runs AUTH_LOCAL_ENABLED=true, so the invite link is the SPA
  // set-password route (ADR-0039/#281), not the Google /start URL.
  const clipboard = await page.evaluate(() => navigator.clipboard.readText());
  expect(clipboard).toContain("/accept?token=");

  // email_sent=true (NoopMailer): no failed-send toast nags the inviter.
  await expect(page.getByText("the email couldn't be sent", { exact: false })).toHaveCount(0);

  // The invite is now listed as pending in the members table.
  await expect(page.getByTestId(`invite-row-${inviteEmail}`)).toBeVisible();
});

// covers: INV-AUTH-31
// Revoke from the members table: a destructive confirm, no Undo, and the link
// that was just copied stops working through the real accept path (the accept
// screen answers "invalid"), not merely disappears from the list.
test("a revoked invite leaves the table and its link stops working", async ({ page, browser }) => {
  await page.goto("/settings");

  const inviteEmail = `e2e-revoked-${Date.now()}@example.com`;
  await page.getByLabel("Email address").fill(inviteEmail);
  await page.getByRole("button", { name: "Send invitation" }).click();
  await page.getByTestId("copy-invite-link").click();
  const link = await page.evaluate(() => navigator.clipboard.readText());
  const acceptPath = link.slice(link.indexOf("/accept"));

  await page.getByTestId(`invite-actions-${inviteEmail}`).click();
  await page.getByTestId(`revoke-${inviteEmail}`).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Revoke invite" }).click();
  await expect(page.getByText("Invite revoked")).toBeVisible();
  await expect(page.getByTestId(`invite-row-${inviteEmail}`)).toHaveCount(0);

  // A fresh, signed-out visitor following the old link gets the invalid notice.
  const visitor = await browser.newContext({ storageState: { cookies: [], origins: [] } });
  const visitorPage = await visitor.newPage();
  await visitorPage.goto(acceptPath);
  await expect(visitorPage.getByTestId("invite-invalid")).toBeVisible();
  await visitor.close();
});
