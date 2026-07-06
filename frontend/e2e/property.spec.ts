import { test, expect } from "@playwright/test";

// Property CRUD through the real UI + backend — an Assets subtype reached at
// /assets/properties. Structurally a mirror of receivable.spec /
// liability.spec (list-row action menu, no detail navigation); included for
// asset-subtype parity. Only the display name is required (type/currency
// default). Anchors on a unique display name; self-cleaning. See ADR-0024.
test("property create → edit → delete round-trip", async ({ page }) => {
  const name = `E2E property ${Date.now()}`;
  const editedName = `${name} edited`;

  await page.goto("/assets/properties");

  // --- Create (display name only; type/currency default) ---
  await page.getByRole("button", { name: "New property" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New property")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByRole("button", { name: "Create" }).click();

  const row = page.getByRole("row", { name: new RegExp(name) });
  await expect(row).toBeVisible();

  // --- Edit (rename via the row action menu) ---
  await row.getByRole("button", { name: "Property actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const editDialog = page.getByRole("dialog");
  await expect(editDialog.getByText("Edit property")).toBeVisible();
  await editDialog.getByLabel("Display name").fill(editedName);
  await editDialog.getByRole("button", { name: "Save changes" }).click();

  const editedRow = page.getByRole("row", { name: new RegExp(editedName) });
  await expect(editedRow).toBeVisible();

  // --- Delete (cleanup) ---
  await editedRow.getByRole("button", { name: "Property actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(editedName)).toHaveCount(0);
});
