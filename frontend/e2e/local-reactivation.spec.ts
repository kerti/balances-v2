import { test, expect } from "@playwright/test";

// Grant clipboard read+write so navigator.clipboard resolves in headless
// Chromium (localhost is a secure context; the permission is the missing piece).
test.use({ permissions: ["clipboard-read", "clipboard-write"] });

// covers: INV-AUTH-20
// Founder-assisted in-app member reactivation (ADR-0039/#283): the no-mail
// recovery affordance. The seeded Carol is DORMANT — a local user with no
// google_sub and no credential, the post-restore state — so the founder Alice
// sees her flagged in the members table and, from her row's ⋯ menu, mints a
// one-time set-password link to relay out-of-band. The link rides the clipboard
// and is never printed (#847).
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

    // The table lists the dormant member Carol with a status; active members
    // (Alice, Bob) are not flagged — they are reachable.
    const carol = page.getByTestId("member-row-carol@example.com");
    await expect(carol).toBeVisible();
    await expect(carol.getByTestId("member-dormant-tag")).toBeVisible();
    await expect(
      page.getByTestId("member-row-alice@example.com").getByTestId("member-dormant-tag"),
    ).toHaveCount(0);

    await page.getByTestId("member-actions-carol@example.com").click();
    await page.getByTestId("reactivate-carol@example.com").click();

    // The one-time link is offered for manual relay — copied, never displayed.
    const dialog = page.getByTestId("reactivation-dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog).not.toContainText("/reset?token=");

    const copyButton = page.getByTestId("copy-reactivation-link");
    await copyButton.click();
    await expect(copyButton).toHaveText("Copied!");

    const clipboard = await page.evaluate(() => navigator.clipboard.readText());
    expect(clipboard).toContain("/reset?token=");
  },
);
