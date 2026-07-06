import { test, expect } from "@playwright/test";

// Position-group CRUD through the real UI + backend for the Receivables group
// (flat — no subtype nav, no transactions): create an entry, edit it, delete
// it, all from the list row's action menu (no detail-page navigation needed,
// mirroring income.spec). Assertions key off a unique display name. The group
// had no E2E coverage; income/lifecycle/snapshot exercise income+assets and
// maturity exercises investments. Self-cleaning — leaves the seed's empty
// receivable list. See ADR-0024.
test("receivable create → edit → delete round-trip", async ({ page }) => {
  const name = `E2E receivable ${Date.now()}`;
  const editedName = `${name} edited`;

  await page.goto("/receivables");

  // --- Create (display name + counterparty required; currency/ownership default) ---
  await page.getByRole("button", { name: "New receivable" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New receivable")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Counterparty").fill("E2E Counterparty");
  await createDialog.getByRole("button", { name: "Create" }).click();

  const row = page.getByRole("row", { name: new RegExp(name) });
  await expect(row).toBeVisible();

  // --- Edit (rename via the row action menu) ---
  await row.getByRole("button", { name: "Receivable actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const editDialog = page.getByRole("dialog");
  await expect(editDialog.getByText("Edit receivable")).toBeVisible();
  await editDialog.getByLabel("Display name").fill(editedName);
  await editDialog.getByRole("button", { name: "Save changes" }).click();

  const editedRow = page.getByRole("row", { name: new RegExp(editedName) });
  await expect(editedRow).toBeVisible();

  // --- Delete (cleanup) ---
  await editedRow.getByRole("button", { name: "Receivable actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(editedName)).toHaveCount(0);
});
