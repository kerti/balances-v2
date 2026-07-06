import { test, expect } from "@playwright/test";

// The last two transaction dialog families (M4.4, ADR-0003): on a stock detail
// page, record a Dividend (CashIncome shape — also covers Coupon/Distribution,
// same dialog) and a Fee (Fee shape, pure-cash variant). Both are cash-only
// shapes; per ADR-0003 neither propagates to bank-account snapshots. trade.spec
// covers the Buy/Sell (quantity-price) shape and maturity.spec covers Maturity,
// so this closes the transaction-family coverage. Anchors on the type cell;
// self-cleaning. See ADR-0024.
test("stock dividend + fee transactions land on the ledger", async ({ page }) => {
  const name = `E2E divfee ${Date.now()}`;

  await page.goto("/investments/stocks");

  // --- Create the stock position ---
  await page.getByRole("button", { name: "New stock" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New stock position")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Ticker").fill("E2EY");
  await createDialog.getByLabel("Exchange").fill("IDX");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();

  // --- Navigate to the detail page ---
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- Record a Dividend (CashIncome shape — amount only) ---
  await page.getByRole("button", { name: "Dividend" }).click();
  const divDialog = page.getByRole("dialog");
  await expect(divDialog.getByRole("heading", { name: "Record Dividend" })).toBeVisible();
  await divDialog.getByLabel("Amount (IDR)").fill("50000");
  await divDialog.getByRole("button", { name: "Record dividend" }).click();
  await expect(page.getByRole("row", { name: /Dividend/ })).toBeVisible();

  // --- Record a Fee (pure-cash variant — quantity/price left blank) ---
  await page.getByRole("button", { name: "Fee" }).click();
  const feeDialog = page.getByRole("dialog");
  await expect(feeDialog.getByRole("heading", { name: "Record Fee" })).toBeVisible();
  await feeDialog.getByLabel("Cash amount (IDR)").fill("25000");
  await feeDialog.getByRole("button", { name: "Record fee" }).click();
  await expect(page.getByRole("row", { name: /Fee/ })).toBeVisible();

  // Both rows coexist on the ledger.
  await expect(page.getByRole("row", { name: /Dividend/ })).toBeVisible();

  // --- Delete (cleanup — returns to the empty list) ---
  await page.getByRole("button", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(name)).toHaveCount(0);
});
