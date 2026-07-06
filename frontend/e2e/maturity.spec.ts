import { test, expect } from "@playwright/test";

// Maturity-flips-status invariant (M4.6, ADR-0009): recording a Maturity
// transaction on a TimeDeposit atomically flips the position to 'matured' on
// the backend (insert + status-flip in one pgx tx). The detail page proves the
// two observable consequences: the StatusBadge flips Active → Matured, and the
// "+ Maturity" create button is gated off afterward (no further transactions on
// a non-active position — the backend would 409). Reached via the transactions
// endpoint, which is why the create hook passes detailKey 'time-deposits' to
// also invalidate the detail cache and refresh the badge + re-gate the row.
// Self-cleaning: deletes the TimeDeposit it creates. See ADR-0024.
test("time deposit maturity flips status to matured and gates the row", async ({ page }) => {
  const name = `E2E TD ${Date.now()}`;
  const statusBadge = page.getByTestId("status-badge");

  await page.goto("/investments/time-deposits");

  // --- Create (term + placement auto-derive maturity; set it explicitly too) ---
  await page.getByRole("button", { name: "New time deposit" }).first().click();
  const createDialog = page.getByRole("dialog");
  await expect(createDialog.getByText("New time deposit")).toBeVisible();
  await createDialog.getByLabel("Display name").fill(name);
  await createDialog.getByLabel("Bank name").fill("E2E Bank");
  await createDialog.getByLabel("Principal").fill("50000000");
  await createDialog.getByLabel("Interest rate (% per year)").fill("4.5");
  await createDialog.getByLabel("Term (months)").fill("12");
  await createDialog.getByLabel("Placement date").fill("2025-01-01");
  await createDialog.getByLabel("Maturity date").fill("2026-01-01");
  await createDialog.getByLabel("Risk profile").selectOption("medium");
  await createDialog.getByRole("button", { name: "Create" }).click();

  // --- Navigate to the detail page ---
  const row = page.getByRole("row", { name: new RegExp(name) });
  await expect(row).toBeVisible();
  await row.getByText(name).click();

  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  // Active position: badge muted-active, Maturity entry available.
  await expect(statusBadge).toHaveText("Active");
  await expect(page.getByRole("button", { name: "Maturity" })).toBeVisible();

  // --- Record Maturity (no_rollover default → both dispositions cash out) ---
  await page.getByRole("button", { name: "Maturity" }).click();
  const matDialog = page.getByRole("dialog");
  await expect(matDialog.getByRole("heading", { name: "Record Maturity" })).toBeVisible();
  await matDialog.getByLabel("Principal (IDR)").fill("50000000");
  await matDialog.getByLabel("Interest (IDR)").fill("2750000");
  await matDialog.getByRole("button", { name: "Record maturity" }).click();

  // Status flips to Matured; the Maturity row lands; the create button is gated.
  await expect(statusBadge).toHaveText("Matured");
  await expect(page.getByRole("row", { name: /Maturity/ })).toBeVisible();
  await expect(page.getByRole("button", { name: "Maturity" })).toHaveCount(0);

  // --- Delete (cleanup — returns to the list, leaving it empty) ---
  await page.getByRole("button", { name: "Delete" }).click();
  const confirm = page.getByRole("alertdialog");
  await confirm.getByRole("button", { name: "Delete" }).click();

  await expect(page.getByText(name)).toHaveCount(0);
});
