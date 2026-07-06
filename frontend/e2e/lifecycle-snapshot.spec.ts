import { test, expect } from "@playwright/test";

// Terminal-flip close snapshot surfaces without a reload (issue #56, the UI face
// of INV-LIFECYCLE-03). A manual Sell on an investment routes through
// PATCH /investments/{id}/lifecycle, which upserts a truthful 0-value close
// snapshot at the termination month server-side (repo/lifecycle.go). The bug:
// the lifecycle mutation refreshed only the list + single-row caches, so the new
// close snapshot stayed invisible in the detail's snapshot list until a manual
// page reload. This drives the real UI — create a stock with an empty snapshot
// list, Sell it, and assert the close snapshot row appears in-place (no reload),
// the regression #56 fixes. Self-cleaning: deletes the stock. See ADR-0024.
// covers: INV-LIFECYCLE-03
test(
  "investment terminal flip surfaces the close snapshot without a reload",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E sell-snap ${Date.now()}`;
    const statusBadge = page.getByTestId("status-badge");

    await page.goto("/investments/stocks");

    // --- Create the stock position (no snapshots yet) ---
    await page.getByRole("button", { name: "New stock" }).first().click();
    const createDialog = page.getByRole("dialog");
    await expect(createDialog.getByText("New stock position")).toBeVisible();
    await createDialog.getByLabel("Display name").fill(name);
    await createDialog.getByLabel("Ticker").fill("E2EX");
    await createDialog.getByLabel("Exchange").fill("IDX");
    await createDialog.getByLabel("Risk profile").selectOption("medium");
    await createDialog.getByRole("button", { name: "Create" }).click();

    // --- Navigate to the detail page ---
    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
    await expect(statusBadge).toHaveText("Active");

    // Snapshot list starts empty — this is the baseline the close snapshot breaks.
    const snapshotCard = page.getByTestId("tour-snapshots");
    await expect(snapshotCard.getByText(/No snapshots yet/)).toBeVisible();
    await expect(snapshotCard.locator("tbody tr")).toHaveCount(0);

    // --- Sell (active → sold; date auto-fills today → close snapshot month) ---
    await page.getByRole("button", { name: "Close", exact: true }).click();
    const sellDialog = page.getByRole("dialog");
    await expect(sellDialog.getByText("Close position")).toBeVisible();
    await sellDialog.getByLabel("Status").selectOption("sold");
    await sellDialog.getByRole("button", { name: "Save" }).click();

    // Status flips to Sold, and — the #56 assertion — the 0-value close snapshot
    // appears in the list in-place: empty state gone, exactly one snapshot row,
    // WITHOUT any page.reload(). Pre-fix this list stayed empty until reload.
    await expect(statusBadge).toHaveText("Sold");
    await expect(snapshotCard.getByText(/No snapshots yet/)).toHaveCount(0);
    await expect(snapshotCard.locator("tbody tr")).toHaveCount(1);

    // --- Delete (cleanup — returns to the list, leaving it empty) ---
    await page.getByRole("button", { name: "Delete" }).click();
    const confirm = page.getByRole("alertdialog");
    await confirm.getByRole("button", { name: "Delete" }).click();

    await expect(page.getByText(name)).toHaveCount(0);
  },
);
