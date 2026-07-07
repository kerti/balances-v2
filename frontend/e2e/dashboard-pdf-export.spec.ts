import { test, expect } from "@playwright/test";

// Dashboard → "Download PDF" (#187, ADR-0044). Seeded Alice is
// auto-authenticated (global-setup), but the seed fixture carries no net
// worth data — the dashboard renders its empty state ("No net worth to show
// yet") until at least one position + snapshot exists, and DashboardHeader
// (which the button lives in) never mounts in that state. Seeds a bank
// account + snapshot first, matching currency-display.spec.ts's pattern;
// self-cleaning — deletes the snapshot and account afterward.
//
// Client-side generation via @react-pdf/renderer triggers a browser download;
// we assert only the suggested filename — no PDF byte/content diffing (see
// ADR-0044's "where automated coverage stops").
test("dashboard PDF export downloads a PDF file", { tag: "@smoke" }, async ({ page }) => {
  const account = `E2E pdf-export account ${Date.now()}`;
  const desc = `E2E pdf-export snapshot ${Date.now()}`;

  // --- Seed a net worth: bank account + one snapshot (IDR) ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const acctDialog = page.getByRole("dialog");
  await acctDialog.getByLabel("Display name").fill(account);
  await acctDialog.getByLabel("Bank name").fill("E2E Bank");
  await acctDialog.getByLabel("Account number").fill("1234567890");
  await acctDialog.getByRole("button", { name: "Create" }).click();

  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
  await page.getByRole("button", { name: "New" }).click();
  const snapDialog = page.getByRole("dialog");
  await snapDialog.getByLabel("Amount (IDR)").fill("12500000");
  await snapDialog.getByLabel("Description (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- Dashboard: download the PDF ---
  await page.getByRole("link", { name: "Dashboard" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Net Worth" })).toBeVisible();

  const btn = page.getByTestId("download-pdf-button");
  await expect(btn).toBeVisible();

  const downloadPromise = page.waitForEvent("download", { timeout: 30_000 });
  await btn.click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toMatch(/^Balances_\d{4}-\d{2}\.pdf$/);

  // --- Cleanup: snapshot, then the account ---
  await page.getByRole("link", { name: "Bank Accounts" }).click();
  await page
    .getByRole("row", { name: new RegExp(account) })
    .getByText(account)
    .click();
  await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
  const row = page.getByRole("row", { name: new RegExp(desc) });
  await row.getByRole("button", { name: "Snapshot actions" }).click();
  await page.getByRole("menuitem", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("No snapshots yet.")).toBeVisible();
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText(account)).toHaveCount(0);
});
