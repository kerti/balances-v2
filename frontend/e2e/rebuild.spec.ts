import { test, expect } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

// Manual report rebuild through the real UI + backend (ADR-0006, M5 slice 4).
// The rebuild row lives in Settings ▸ Reports (#951); its per-month button only
// renders once there's a report to target, so this first records a snapshot on a fresh bank account, then exercises both
// rebuild scopes — per-month (surgical) and rebuild-all (engine/FX changes that
// ripple across history) — asserting each POST returns 200 and the dashboard
// stays rendered with no error. Self-cleaning: deletes the snapshot and the
// parent account, leaving the seed's empty lists. See ADR-0024.
test("settings rebuild: latest month + rebuild-all", async ({ page }) => {
  const account = `E2E rebuild account ${Date.now()}`;
  const desc = `E2E rebuild snapshot ${Date.now()}`;

  // --- Seed a net worth: bank account + one snapshot ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const acctDialog = page.getByRole("dialog");
  await acctDialog.getByLabel("Display name").fill(account);
  await acctDialog.getByTestId("entry-type-options").getByRole("radio").first().click();
  await acctDialog.getByLabel("Bank name").fill("E2E Bank");
  await acctDialog.getByLabel("Account number").fill("1234567890");
  await acctDialog.getByRole("button", { name: "Create" }).click();
  await backToListAfterCreate(page);

  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();

  await page.getByRole("button", { name: "Record snapshot" }).click();
  const snapDialog = page.getByRole("dialog");
  await expect(snapDialog.getByText("Record monthly snapshot")).toBeVisible();
  await snapDialog.getByLabel("Amount (IDR)").fill("12500000");
  await snapDialog.getByLabel("Note (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- A report now exists → Settings ▸ Reports offers the per-month rebuild ---
  await page.goto("/settings");
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();

  // --- Rebuild the latest month (per-month scope: /api/reports/YYYY-MM/rebuild) ---
  const monthResp = page.waitForResponse(
    (r) => /\/api\/reports\/\d{4}-\d{2}\/rebuild$/.test(r.url()) && r.request().method() === "POST",
  );
  await page.getByTestId("rebuild-month").click();
  expect((await monthResp).status()).toBe(200);

  // --- Rebuild all months (household scope: /api/reports/rebuild) ---
  const allResp = page.waitForResponse(
    (r) => /\/api\/reports\/rebuild$/.test(r.url()) && r.request().method() === "POST",
  );
  await page.getByTestId("rebuild-all").click();
  expect((await allResp).status()).toBe(200);

  // Settings stayed healthy through both rebuilds.
  await expect(page.getByText("Rebuild failed — try again.")).toHaveCount(0);
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();

  // --- Cleanup: delete the snapshot, then the parent account ---
  await page.getByRole("link", { name: "Bank accounts" }).click();
  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();

  const row = page.getByRole("row", { name: new RegExp(desc) });
  await row.getByRole("button", { name: "Snapshot actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await expect(page.getByText("No snapshots yet.")).toBeVisible();

  await page.getByTestId("position-menu-trigger").click();

  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(account)).toHaveCount(0);
});
