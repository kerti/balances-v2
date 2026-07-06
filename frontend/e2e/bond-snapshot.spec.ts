import { test, expect } from "@playwright/test";

// Accrued-interest snapshot shape through the real UI + backend (ADR-0022, the
// third snapshot family): on a bond detail page, record a snapshot carrying
// total value + accrued-interest breakdown, then delete it. This is the only
// snapshot dialog family the other specs don't touch — snapshot.spec covers
// amount-only and trade.spec covers quantity-price. Bond create requires
// display name / issuer / face value / coupon rate / maturity date. Anchors on
// a unique description; self-cleaning. See ADR-0024.
test("bond accrued-interest snapshot create → delete", async ({ page }) => {
  const name = `E2E bond ${Date.now()}`;
  const desc = `E2E accrued ${Date.now()}`;

  await page.goto("/investments/bonds");

  // --- Create the bond position ---
  await page.getByRole("button", { name: "New bond" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New bond position")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Issuer").fill("E2E Treasury");
  await createDialog.getByLabel("Face value").fill("1000000");
  await createDialog.getByLabel("Coupon rate (% per year)").fill("6.5");
  await createDialog.getByLabel("Maturity date").fill("2030-01-01");
  await createDialog.getByLabel("Placement date").fill("2024-01-01");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();

  // --- Navigate to the detail page ---
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // --- Record an accrued-interest snapshot (total value + accrued) ---
  await page.getByRole("button", { name: "New" }).click();
  const snapDialog = page.getByRole("dialog");
  await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
  await snapDialog.getByLabel("Total value (IDR)").fill("1010000");
  await snapDialog.getByLabel("Accrued (IDR)").fill("10000");
  await snapDialog.getByLabel("Description (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();

  const row = page.getByRole("row", { name: new RegExp(desc) });
  await expect(row).toBeVisible();

  // --- Delete the snapshot (table returns to its empty state) ---
  await row.getByRole("button", { name: "Snapshot actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  const snapConfirm = page.getByRole("alertdialog");
  await snapConfirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(desc)).toHaveCount(0);
  await expect(page.getByText("No snapshots yet.")).toBeVisible();

  // --- Delete the parent bond (cleanup — returns to the empty list) ---
  await page.getByRole("button", { name: "Delete" }).click();
  const bondConfirm = page.getByRole("alertdialog");
  await bondConfirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(name)).toHaveCount(0);
});
