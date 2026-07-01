import { test, expect } from "@playwright/test";

// Grant clipboard read+write so navigator.clipboard resolves in headless
// Chromium (localhost is a secure context; the permission is the missing piece).
test.use({ permissions: ["clipboard-read", "clipboard-write"] });

// covers: INV-AUTH-20
// Founder-assisted in-app member reactivation (ADR-0039/#283): the no-mail
// recovery affordance. The seeded Carol is DORMANT — a local user with no
// google_sub and no credential, the post-restore state — so the founder Alice
// sees her in the reactivation card and mints a one-time set-password link,
// surfaced on screen to relay out-of-band (the same copy-link shape as invites).
//
// This spec owns the SPA wiring. Minting the link is non-destructive (Carol stays
// dormant until she actually sets a password), so the spec is idempotent across
// reruns. The full dormant → set-password → login round-trip is owned by the
// backend handler tests against a real DB (reactivation_test.go), which also pin
// the founder-only and dormant-only scoping.
test(
  "founder reactivates a dormant member with a copyable set-password link",
  { tag: "@smoke" },
  async ({ page }) => {
    await page.goto("/settings");

    // The card lists the dormant member Carol; active members (Alice, Bob) don't
    // appear — they are reachable, not awaiting a first credential.
    const carol = page.getByTestId("dormant-member-carol@example.com");
    await expect(carol).toBeVisible();

    await page.getByTestId("reactivate-carol@example.com").click();

    // The one-time link is surfaced for manual relay — a /reset set-password link.
    const link = page.getByTestId("reactivation-link");
    await expect(link).toBeVisible();
    const url = await link.innerText();
    expect(url).toContain("/reset?token=");

    const copyButton = page.getByTestId("copy-reactivation-link");
    await expect(copyButton).toBeVisible();
    await copyButton.click();
    await expect(copyButton).toHaveText("Copied!");

    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboard).toBe(url);
  },
);
