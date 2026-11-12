import { test, expect } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

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
  await expect(createDialog.getByRole("heading", { name: "New stock" })).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Ticker").fill("E2EX");
  await createDialog.getByLabel("Exchange").fill("IDX");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);

  // --- Navigate to the detail page ---
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- Record a Buy: 100 sh @ 8500 (quantity-price transaction shape) ---
  await page.getByRole("button", { name: "Buy" }).click();
  const buyDialog = page.getByRole("dialog");
  await expect(buyDialog.getByRole("heading", { name: "Record buy" })).toBeVisible();
  await buyDialog.getByLabel("Quantity (sh)").fill("100");
  await buyDialog.getByLabel("Price per unit (IDR)").fill("8500");
  await buyDialog.getByRole("button", { name: "Record buy" }).click();

  // Buy row lands; no reconciliation warning yet (no snapshot to compare).
  await expect(page.getByRole("row", { name: /Buy/ })).toBeVisible();
  await expect(page.getByText(/match ledger total/)).toHaveCount(0);

  // --- Add a snapshot whose quantity (90) disagrees with the ledger (100) ---
  await page.getByRole("button", { name: "Record snapshot" }).click();
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

  await expect(page.getByRole("main").getByText(name)).toHaveCount(0);
});

// A Buy never writes the holding's snapshot (#830): its toast and the mismatch
// banner both offer "Update {Mon} holding →", which only opens Record snapshot
// prefilled (ledger quantity 40 + 10 = 50, latest price); nothing is saved until
// the user presses Save, and saving clears the banner. Self-cleaning.
// covers: INV-SNAPSHOTS-18
test("stock buy offers Update holding, which prefills Record snapshot and saves only on confirm", async ({
  page,
}) => {
  const name = `E2E holding ${Date.now()}`;

  await page.goto("/investments/stocks");
  await page.getByRole("button", { name: "New stock" }).first().click();
  const createDialog = page.getByRole("dialog");
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Ticker").fill("E2EH");
  await createDialog.getByLabel("Exchange").fill("IDX");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- A 40 sh holding: Buy 40, and a snapshot that agrees with the ledger ---
  await page.getByRole("button", { name: "Buy" }).click();
  let dialog = page.getByRole("dialog");
  // The trade dialog says up front that the holding and bank are updated separately.
  await expect(dialog.getByTestId("trade-separate-note")).toContainText(
    "This records the trade. Update the holding in this month's snapshot",
  );
  await dialog.getByLabel("Quantity (sh)").fill("40");
  await dialog.getByLabel("Price per unit (IDR)").fill("8500");
  await dialog.getByRole("button", { name: "Record buy" }).click();
  await expect(page.getByRole("row", { name: /Buy/ })).toBeVisible();

  await page.getByRole("button", { name: "Record snapshot" }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("Quantity", { exact: true }).fill("40");
  await dialog.getByLabel("Price per unit (IDR)").fill("8500");
  await dialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByTestId("reconcile-banner")).toHaveCount(0);

  // --- Buy 10 more: the ledger says 50, the snapshot still says 40 ---
  await page.getByRole("button", { name: "Buy" }).click();
  dialog = page.getByRole("dialog");
  await dialog.getByLabel("Quantity (sh)").fill("10");
  await dialog.getByLabel("Price per unit (IDR)").fill("9000");
  await dialog.getByRole("button", { name: "Record buy" }).click();

  const banner = page.getByTestId("reconcile-banner");
  await expect(banner).toBeVisible();

  // The toast offers the shortcut; clicking it opens Record snapshot prefilled
  // with 50 and the latest snapshot's price - and writes nothing. The first
  // Buy's toast can still be up; the prefill reads the ledger at click time, so
  // either toast's action gives 50.
  await page
    .getByTestId("toast-trade-added")
    .getByRole("button", { name: /^Update .+ holding/ })
    .first()
    .click();
  dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Record monthly snapshot")).toBeVisible();
  await expect(dialog.getByLabel("Quantity", { exact: true })).toHaveValue("50");
  await expect(dialog.getByLabel("Price per unit (IDR)")).toHaveValue(/^8,?500/);
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(banner).toBeVisible(); // cancelling saved nothing

  // The mismatch banner's action does the same; Save is what writes.
  await banner.getByTestId("reconcile-update-holding").click();
  dialog = page.getByRole("dialog");
  await expect(dialog.getByLabel("Quantity", { exact: true })).toHaveValue("50");
  await dialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByTestId("reconcile-banner")).toHaveCount(0);

  // --- Delete (cleanup) ---
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(name)).toHaveCount(0);
});
