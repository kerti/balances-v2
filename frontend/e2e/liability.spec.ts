import { test, expect } from "@playwright/test";

// Position-group CRUD through the real UI + backend for the Liabilities group
// (Personal / Institutional subtypes): create a personal
// liability, edit it, delete it, all from the list row's action menu. Closes
// the last position group with no E2E coverage. Only display name + counterparty
// are required; principal/rate/term/dates are optional. Anchors on a unique
// display name; self-cleaning. See ADR-0024.
test("liability create → edit → delete round-trip", async ({ page }) => {
  const name = `E2E liability ${Date.now()}`;
  const editedName = `${name} edited`;

  await page.goto("/liabilities/personal");

  // --- Create (display name + counterparty required; subtype fixed to Personal) ---
  await page.getByRole("button", { name: "New liability" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New liability")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Counterparty").fill("E2E Counterparty");
  await createDialog.getByRole("button", { name: "Create" }).click();

  const row = page.getByRole("row", { name: new RegExp(name) });
  await expect(row).toBeVisible();

  // --- Edit (rename via the row action menu) ---
  await row.getByRole("button", { name: "Liability actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const editDialog = page.getByRole("dialog");
  await expect(editDialog.getByText("Edit liability")).toBeVisible();
  await editDialog.getByLabel("Display name").fill(editedName);
  await editDialog.getByRole("button", { name: "Save changes" }).click();

  const editedRow = page.getByRole("row", { name: new RegExp(editedName) });
  await expect(editedRow).toBeVisible();

  // --- Delete (cleanup) ---
  await editedRow.getByRole("button", { name: "Liability actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(editedName)).toHaveCount(0);
});
