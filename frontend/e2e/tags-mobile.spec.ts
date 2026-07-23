import { test, expect } from "@playwright/test";

// Tag breakdown mobile divergence (#509, ADR-0050 B2c "wide table → stacked
// cards"). The /tags report splits at the renderer: useIsMobile (768px) mounts
// TagBreakdownCards on phones and the wide holdings/liabilities/net table on
// desktop, both under the shared `tag-breakdown-<currency>` testid. This @smoke
// asserts the correct renderer mounts at each width and the ADR-0050 a11y floor
// holds on the card — the net value (the figure the household member came for)
// reads with no horizontal page scroll and the pie-inclusion toggle clears
// 44px. Deep per-field assertions stay in the nightly suite; the desktop
// assign/report flow lives in tags.spec.ts.
//
// Seeds a positive-holdings tagged position (bank account + one snapshot + a
// tag assignment) so a breakdown actually renders, and self-cleans afterward.
//
// covers: INV-PRESENTATION-08
test(
  "tags breakdown mounts the card renderer and holds the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const tagName = `E2E Tag ${Date.now()}`;
    const account = `E2E Tagged Acct ${Date.now()}`;
    const desc = `E2E snapshot ${Date.now()}`;

    // --- Create the tag on /tags ---
    await page.goto("/tags");
    await page.getByTestId("new-tag-name").fill(tagName);
    await page.getByTestId("add-tag").click();
    await expect(page.getByTestId("tag-list").getByText(tagName)).toBeVisible();

    // --- Create a bank account and give it a balance (holdings) ---
    await page.goto("/assets/bank-accounts");
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    await acctDialog.getByLabel("Account number").fill("1234567890");
    await acctDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
    await page.getByRole("button", { name: "New" }).click();
    const snapDialog = page.getByRole("dialog");
    await snapDialog.getByLabel("Amount (IDR)").fill("50000000");
    await snapDialog.getByLabel("Description (optional)").fill(desc);
    await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
    await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

    // --- Assign the tag via the detail-screen control (await the PUT) ---
    const assigned = page.waitForResponse(
      (r) => r.url().includes("/api/tags/assignments") && r.request().method() === "PUT" && r.ok(),
    );
    await page.getByTestId("tag-select").selectOption({ label: tagName });
    await assigned;

    // --- Desktop: the wide table renderer mounts under the currency section ---
    await page.goto("/tags");
    const section = page.getByTestId("tag-breakdown-IDR");
    await expect(section).toBeVisible();
    await expect(section.getByRole("table")).toBeVisible();
    await expect(page.getByTestId("tag-breakdown-cards-IDR")).toHaveCount(0);

    // --- Phone width: the renderer flips from table to cards ---
    // At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/tags");
    const cards = page.getByTestId("tag-breakdown-cards-IDR");
    await expect(cards).toBeVisible();
    await expect(page.getByTestId("tag-breakdown-IDR").getByRole("table")).toHaveCount(0);

    // Primary value reachable: the net figure reads on the card, in the viewport.
    const net = page.getByTestId("tag-breakdown-net").first();
    await expect(net).toBeVisible();
    await expect(net).toBeInViewport();

    // No horizontal page scroll — the stacked cards fit the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the pie-inclusion toggle (label around the checkbox +
    // badge) clears 44px.
    const toggle = cards.locator("label").first();
    const box = await toggle.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: delete the account (drops the holdings), then the tag ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/assets/bank-accounts");
    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();

    await page.goto("/tags");
    await page.getByTestId("delete-tag").click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: /delete/i })
      .click();
    await expect(page.getByTestId("tag-list").getByText(tagName)).toHaveCount(0);
  },
);
