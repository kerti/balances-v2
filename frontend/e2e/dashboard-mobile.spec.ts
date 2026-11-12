import { test, expect } from "@playwright/test";
import { backToListAfterCreate } from "./landing";

// Dashboard mobile a11y floor (#507, ADR-0050 B2a). The dashboard is already a
// single-column stack, so the doctrine's "grid → single-column stack" transform
// lands as pure CSS reflow — no useIsMobile renderer split. This @smoke asserts
// the reflow holds the ADR-0050 floor at phone width: the primary net-worth
// value is reachable with no horizontal page scroll, and the toolbar's tap
// targets clear 44px. Deep per-control assertions stay out of the smoke tier.
//
// Seeds a net worth first (a bank account + one snapshot) — like
// dashboard-pdf-export.spec.ts, DashboardHeader only mounts once real data
// exists — and self-cleans afterward.
//
// covers: INV-PRESENTATION-08
test("dashboard holds the mobile a11y floor at 390px", { tag: "@smoke" }, async ({ page }) => {
  const account = `E2E mobile account ${Date.now()}`;
  const desc = `E2E mobile snapshot ${Date.now()}`;

  // --- Seed a net worth: bank account + one snapshot (IDR) ---
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const acctDialog = page.getByRole("dialog");
  await acctDialog.getByLabel("Display name").fill(account);
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
  await snapDialog.getByLabel("Amount (IDR)").fill("12500000");
  await snapDialog.getByLabel("Note (optional)").fill(desc);
  await snapDialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("row", { name: new RegExp(desc) })).toBeVisible();

  // --- Phone width, then the dashboard ---
  // At <768px the shell collapses the sidebar into a hamburger drawer
  // (ADR-0025), so the "Dashboard" nav link isn't directly clickable — navigate
  // to the dashboard route ("/") by URL instead.
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: "Net worth" })).toBeVisible();

  // Primary value reachable: the headline is visible and inside the viewport
  // (not pushed off-screen by a crammed header row).
  const headline = page.getByTestId("dashboard-headline");
  await expect(headline).toBeVisible();
  await expect(headline).toBeInViewport();

  // No horizontal page scroll — the reflow keeps the layout within the viewport.
  const overflow = await page.evaluate(() => {
    const el = document.scrollingElement ?? document.documentElement;
    return el.scrollWidth - el.clientWidth;
  });
  expect(overflow).toBeLessThanOrEqual(1);

  // Tap-target floor: the month-picker trigger sizes up to ≥44px on mobile.
  const monthPicker = page.getByTestId("month-picker-trigger");
  await expect(monthPicker).toBeVisible();
  const box = await monthPicker.boundingBox();
  expect(box).not.toBeNull();
  expect(box!.height).toBeGreaterThanOrEqual(44);

  // Range chips (#852): 5Y is the default, each chip clears the tap floor, the
  // pressed state is exposed, and the choice survives a reload.
  await expect(page.getByTestId("chart-range-5Y")).toHaveAttribute("aria-pressed", "true");
  const chipBox = await page.getByTestId("chart-range-1Y").boundingBox();
  expect(chipBox).not.toBeNull();
  expect(chipBox!.height).toBeGreaterThanOrEqual(44);
  await page.getByTestId("chart-range-3Y").click();
  await expect(page.getByTestId("chart-range-3Y")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("chart-range-5Y")).toHaveAttribute("aria-pressed", "false");
  await page.reload();
  await expect(page.getByTestId("chart-range-3Y")).toHaveAttribute("aria-pressed", "true");
  await page.getByTestId("chart-range-5Y").click();

  // --- Cleanup: snapshot, then the account (desktop width for the wide table) ---
  await page.setViewportSize({ width: 1280, height: 900 });
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
  await page.getByRole("button", { name: "Delete" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
  await expect(page.getByRole("main").getByText(account)).toHaveCount(0);
});
