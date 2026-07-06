import { test, expect } from "@playwright/test";

// Investment trade + soft reconciliation through the real UI + backend (M4.4,
// ADR-0003): create a stock, record a Buy (quantity-price transaction shape),
// then add a quantity-price snapshot whose quantity disagrees with the ledger
// so the display-only reconciliation warning fires. Statements remain the
// source of truth — the mismatch is a data-entry flag, never a write block, so
// the snapshot still saves and the warning is the only consequence. Exercises
// the last untouched dialog family (quantity-price trade + snapshot). Anchors
// on a unique display name; self-cleaning. See ADR-0024.
test("stock buy + mismatched snapshot raises the reconciliation warning", async ({ page }) => {
  const name = `E2E stock ${Date.now()}`;

  await page.goto("/investments/stocks");

  // --- Create the stock position ---
  await page.getByRole("button", { name: "New stock" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New stock position")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Ticker").fill("E2EX");
  await createDialog.getByLabel("Exchange").fill("IDX");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();

  // --- Navigate to the detail page ---
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- Record a Buy: 100 sh @ 8500 (quantity-price transaction shape) ---
  await page.getByRole("button", { name: "Buy" }).click();
  const buyDialog = page.getByRole("dialog");
  await expect(buyDialog.getByRole("heading", { name: "Record Buy" })).toBeVisible();
  await buyDialog.getByLabel("Quantity (sh)").fill("100");
  await buyDialog.getByLabel("Price per unit (IDR)").fill("8500");
  await buyDialog.getByRole("button", { name: "Record buy" }).click();

  // Buy row lands; no reconciliation warning yet (no snapshot to compare).
  await expect(page.getByRole("row", { name: /Buy/ })).toBeVisible();
  await expect(page.getByText(/match ledger total/)).toHaveCount(0);

  // --- Add a snapshot whose quantity (90) disagrees with the ledger (100) ---
  await page.getByRole("button", { name: "New" }).click();
  const snapDialog = page.getByRole("dialog");
  await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
  await snapDialog.getByLabel("Quantity", { exact: true }).fill("90");
  await snapDialog.getByLabel("Price per unit (IDR)").fill("8500");
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();

  // Warning fires (display-only); the snapshot still saved despite the mismatch.
  await expect(page.getByText(/Latest snapshot quantity \(90 sh\)/)).toBeVisible();
  await expect(page.getByText(/match ledger total/)).toBeVisible();

  // --- Delete (cleanup — returns to the empty list) ---
  await page.getByRole("button", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(name)).toHaveCount(0);
});
