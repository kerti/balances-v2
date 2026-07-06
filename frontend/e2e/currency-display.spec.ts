import { test, expect } from "@playwright/test";

// Side-by-side currency display through the real UI + backend (Q15c, ADR-0010,
// M5 slice 4). The monthly report is stored in the reporting currency; the
// dashboard projects the headline net worth into a second currency at the
// month's FX rate (carry-forward). This: seeds a net worth (account + snapshot),
// enables multi-currency + enters a USD rate in Settings, then picks "Also in:
// USD" on the dashboard and asserts the "≈" approximation renders a real
// amount. Self-cleaning — deletes the snapshot, account, and rate, and turns
// multi-currency back off, restoring the seed state. See ADR-0024.
test("dashboard side-by-side currency: project net worth into USD", async ({ page }) => {
  const account = `E2E fx account ${Date.now()}`;
  const desc = `E2E fx snapshot ${Date.now()}`;
  const now = new Date();
  const month = `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;

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
  await page.getByRole("button", { name: "← Back" }).click();

  // --- Settings: enable multi-currency, then enter a USD rate for this month ---
  await page.getByRole("link", { name: "Settings" }).click();
  // Controlled checkbox: the toggle is async (mutation → session refetch), so
  // click and let the FX-rates card's appearance confirm the flip stuck.
  await page.getByLabel("Enable multi-currency tracking").click();
  await expect(page.getByText("Exchange rates", { exact: true })).toBeVisible();
  await page.getByLabel("Month").fill(month);
  await page.getByLabel("Currency", { exact: true }).fill("USD");
  await page.getByLabel("Rate").fill("16000");
  await page.getByRole("button", { name: "Add rate" }).click();
  await expect(page.getByRole("cell", { name: "USD" })).toBeVisible();

  // --- Dashboard: pick "Also in: USD" → headline gains the ≈ approximation ---
  await page.getByRole("link", { name: "Dashboard" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Net Worth" })).toBeVisible();
  await page.getByTestId("dashboard-secondary-currency").selectOption("USD");

  const approx = page.getByTestId("dashboard-secondary-amount");
  await expect(approx).toBeVisible();
  // A real conversion, not the "no rate yet" fallback.
  await expect(approx).not.toContainText("no USD rate yet");

  // --- Cleanup: snapshot + account, then the rate, then turn multi-currency off ---
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

  await page.getByRole("link", { name: "Settings" }).click();
  await page.getByRole("row", { name: /USD/ }).getByRole("button", { name: "Delete" }).click();
  await expect(page.getByText("No rates entered yet.")).toBeVisible();
  await page.getByLabel("Enable multi-currency tracking").click();
  await expect(page.getByText("Exchange rates", { exact: true })).toHaveCount(0);
});
