import { test, expect } from "@playwright/test";

// Group-hub mobile a11y floor (#510, ADR-0050 B3a). The three hubs — AssetsHome,
// InvestmentsHome, LiabilitiesHome — are already single-column chart stacks
// (only InvestmentsHome's pie pair is a grid, already md:grid-cols-2), so the
// doctrine's "grid → single-column stack" transform lands as pure CSS reflow —
// no useIsMobile renderer split. This @smoke asserts the reflow holds the
// ADR-0050 floor at phone width across all three hubs: no horizontal page
// scroll, and the bulk-entry button's tap target clears 44px.
//
// Seeds a net worth first (a bank account + one snapshot) so AssetsHome renders
// its per-currency headline; Investments/Liabilities have no seeded positions
// but their header + bulk-entry button render unconditionally. Self-cleans.
//
// covers: INV-PRESENTATION-08
test("group hubs hold the mobile a11y floor at 390px", { tag: "@smoke" }, async ({ page }) => {
  const account = `E2E hub mobile account ${Date.now()}`;
  const desc = `E2E hub mobile snapshot ${Date.now()}`;

  // --- Seed a net worth: bank account + one snapshot (IDR) ---
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
  await snapDialog.getByLabel("Amount (IDR)").fill("12500000");
  await snapDialog.getByLabel("Description (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- Phone width ---
  // At <768px the shell collapses the sidebar into a hamburger drawer (ADR-0025),
  // so hub nav links aren't directly clickable — navigate by URL instead.
  await page.setViewportSize({ width: 390, height: 844 });

  const noHorizontalOverflow = async () => {
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);
  };

  const tapTargetFloor = async (testId: string) => {
    const btn = page.getByTestId(testId);
    await expect(btn).toBeVisible();
    const box = await btn.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
  };

  // --- Assets hub: seeded, so the headline renders and must sit in-viewport ---
  await page.goto("/assets");
  await expect(page.getByRole("heading", { level: 1, name: "Assets" })).toBeVisible();
  const assetsHeadline = page.getByTestId("home-total");
  await expect(assetsHeadline).toBeVisible();
  await expect(assetsHeadline).toBeInViewport();
  await noHorizontalOverflow();
  await tapTargetFloor("assets-enter-month");

  // --- Investments hub: two bulk-entry buttons; the primary sizes to the floor ---
  await page.goto("/investments");
  await expect(page.getByRole("heading", { level: 1, name: "Investments" })).toBeVisible();
  await noHorizontalOverflow();
  await tapTargetFloor("investments-enter-prices");

  // --- Liabilities hub ---
  await page.goto("/liabilities");
  await expect(page.getByRole("heading", { level: 1, name: "Liabilities" })).toBeVisible();
  await noHorizontalOverflow();
  await tapTargetFloor("liabilities-enter-month");

  // --- Cleanup: snapshot, then the account (desktop width for the wide table) ---
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/assets/bank-accounts");
  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(desc) });
  await row.getByRole("button", { name: "Snapshot actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("No snapshots yet.")).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText(account)).toHaveCount(0);
});
