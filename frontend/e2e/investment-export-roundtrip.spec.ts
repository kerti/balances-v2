import { readFileSync } from "node:fs";
import { test, expect } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

const XLSX_MIME = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";

// Export → re-import round-trip for the investment position workbook (#87,
// parent #51). On a stock detail page (representative of the five investment
// subtype exports) we record a Buy transaction and a quantity-price snapshot,
// download the export — which now carries a Detail + Snapshots + Transactions
// workbook (ADR-0023) — then feed that exact file back through the snapshot-
// import dialog. The import reads only the Snapshots sheet, so the Detail +
// Transactions sheets are ignored and the one month re-classifies as unchanged
// (not an insert, not a replacement): a true round trip. Anchors on a unique display name +
// description; self-cleaning. See ADR-0024.
test("stock export re-imports flawlessly", async ({ page }) => {
  const name = `E2E export stock ${Date.now()}`;
  const desc = `E2E export snap ${Date.now()}`;

  await page.goto("/investments/stocks");

  // --- Create the parent stock position ---
  await page.getByRole("button", { name: "New stock" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New stock position")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Ticker").fill("E2EX");
  await createDialog.getByLabel("Exchange").fill("IDX");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);

  // --- Open the detail page ---
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- Record a Buy so the export's Transactions sheet has a real ledger row ---
  await page.getByRole("button", { name: "Buy" }).click();
  const buyDialog = page.getByRole("dialog");
  await expect(buyDialog.getByRole("heading", { name: "Record buy" })).toBeVisible();
  await buyDialog.getByLabel("Quantity (sh)").fill("100");
  await buyDialog.getByLabel("Price per unit (IDR)").fill("8500");
  await buyDialog.getByRole("button", { name: "Record buy" }).click();
  await expect(page.getByRole("row", { name: /Buy/ })).toBeVisible();

  // --- Record one quantity-price snapshot (month defaults to current) ---
  await page.getByRole("button", { name: "Record snapshot" }).click();
  const snapDialog = page.getByRole("dialog");
  await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
  await snapDialog.getByLabel("Quantity", { exact: true }).fill("100");
  await snapDialog.getByLabel("Price per unit (IDR)").fill("8500");
  await snapDialog.getByLabel("Description (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- Export: the button is a plain anchor download; capture the file ---
  const downloadPromise = page.waitForEvent("download");
  await page.getByTestId("stock-export").click();
  const download = await downloadPromise;
  const filename = download.suggestedFilename();
  expect(filename).toMatch(/\.xlsx$/);

  // Read the bytes and re-attach them under the real filename + MIME — same
  // dance as the property round trip: download.path() is an extensionless temp
  // file the .xlsx validator (lib/importDrop) would reject.
  const buffer = readFileSync(await download.path());

  // --- Feed the exported file straight back through the import dialog ---
  await page.getByTestId("import-snapshots-trigger").click();
  await page
    .getByTestId("import-file-input")
    .setInputFiles({ name: filename, mimeType: XLSX_MIME, buffer });
  await expect(page.getByTestId("import-selected-file")).toBeVisible();

  // Picking the file runs the dry-run (no Check button, #829). The file is
  // clean and the one month matches the stored snapshot exactly — quantity,
  // price, date and note — so it reads "unchanged": nothing to write, Import
  // stays disabled. That also proves the Detail + Transactions sheets are
  // ignored and only the Snapshots sheet drives the import.
  const result = page.getByTestId("import-result");
  await expect(result).toBeVisible();
  await expect(page.getByTestId("import-all-unchanged")).toContainText(
    "Nothing to import — every month already matches",
  );
  await expect(page.getByTestId("import-errors")).toHaveCount(0);
  await expect(page.getByTestId("import-commit-btn")).toBeDisabled();
  await expect(page.getByTestId("import-check-btn")).toHaveCount(0);

  await page.getByRole("dialog").getByRole("button", { name: "Cancel" }).click();

  // The snapshot survived the round trip unchanged.
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- Delete the parent stock (cleanup — returns to the empty list) ---
  await page.getByRole("button", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(name)).toHaveCount(0);
});
