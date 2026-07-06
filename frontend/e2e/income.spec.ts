import { test, expect } from "@playwright/test";

// Full income write-flow through the real UI + backend: create an entry, edit
// it, delete it. Navigation is by URL (React Router, ADR-0025). Assertions key
// off a unique
// description — the amount renders IDR-formatted, so it's a poor anchor. The
// test is self-cleaning: it deletes what it creates, leaving income empty as
// the seed does. See ADR-0024.
test("income create → edit → delete round-trip", async ({ page }) => {
  const desc = `E2E income ${Date.now()}`;
  const editedDesc = `${desc} edited`;

  await page.goto("/income");

  // --- Create ---
  await page.getByRole("button", { name: "New income" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New income")).toBeVisible();
  await createDialog.getByLabel("Amount").fill("15000000");
  await createDialog.getByLabel("Category").selectOption("salary");
  await createDialog.getByLabel("Description (optional)").fill(desc);
  await createDialog.getByRole("button", { name: "Create" }).click();

  const row = page.getByRole("row", { name: new RegExp(desc) });
  await expect(row).toBeVisible();

  // --- Edit ---
  await row.getByRole("button", { name: "Income actions" }).click();
  await page.getByRole("menuitem", { name: "Edit" }).click();
  const editDialog = page.getByRole("dialog");
  await expect(editDialog.getByText("Edit income")).toBeVisible();
  await editDialog.getByLabel("Description (optional)").fill(editedDesc);
  await editDialog.getByRole("button", { name: "Save changes" }).click();

  const editedRow = page.getByRole("row", { name: new RegExp(editedDesc) });
  await expect(editedRow).toBeVisible();

  // --- Delete ---
  await editedRow.getByRole("button", { name: "Income actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(editedDesc)).toHaveCount(0);
});
