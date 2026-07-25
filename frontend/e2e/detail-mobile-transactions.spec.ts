import { test, expect } from "@playwright/test";

// Detail-page mobile divergence for the investment transaction-ledger shape
// (#538, ADR-0051 Phase B / ADR-0050 "wide table → stacked cards"). Sibling to
// the qty×price (#536) and accrued (#537) @smokes: all five investment pages
// (Stock, MutualFund, Gold, Bond, TimeDeposit) share one `TransactionRow`/`Card`
// pair off the generic shell, so the flip is proven once here on Stock, the
// representative subtype. useIsMobile (768px) mounts the transaction card list on
// phones and the wide table on desktop; this asserts the flip and the ADR-0050
// a11y floor — the cash impact reads with no horizontal page scroll and the row
// ⋮ action clears 44px. Deep per-shape assertions live nightly.
//
// Seeds a stock + one Buy transaction, exercises both widths, self-cleans.
//
// covers: INV-PRESENTATION-08
test(
  "transaction ledger mounts the card renderer and holds the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E txn mobile ${Date.now()}`;

    // --- Seed a stock + one Buy (desktop width) ---
    await page.goto("/investments/stocks");
    await page.getByRole("button", { name: "New stock" }).first().click();
    const createDialog = page.getByRole("dialog");
    await createDialog.getByLabel("Display name").fill(name);
    await createDialog.getByLabel("Ticker").fill("E2EX");
    await createDialog.getByLabel("Exchange").fill("IDX");
    await createDialog.getByLabel("Risk profile").selectOption("medium");
    await createDialog.getByRole("button", { name: "Create" }).click();

    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

    await page.getByRole("button", { name: "Buy" }).click();
    const buyDialog = page.getByRole("dialog");
    await expect(buyDialog.getByRole("heading", { name: "Record Buy" })).toBeVisible();
    await buyDialog.getByLabel("Quantity (sh)").fill("100");
    await buyDialog.getByLabel("Price per unit (IDR)").fill("8500");
    await buyDialog.getByRole("button", { name: "Record buy" }).click();

    // Desktop: the Buy renders in the wide transaction table.
    await expect(page.getByTestId("tour-transactions-table")).toBeVisible();

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await expect(page.getByTestId("tour-transactions-cards")).toBeVisible();
    await expect(page.getByTestId("tour-transactions-table")).toHaveCount(0);

    // Primary value reachable: the cash impact reads on the card once scrolled to
    // (the stock's headline/details/snapshot section push the ledger below the
    // fold at phone height — reachable by vertical scroll is the a11y bar).
    const amount = page.getByTestId("transaction-amount").first();
    await expect(amount).toBeVisible();
    await amount.scrollIntoViewIfNeeded();
    await expect(amount).toBeInViewport();

    // No horizontal page scroll — the stacked card fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the card's ⋮ action clears 44px.
    const actions = page.getByRole("button", { name: "Transaction actions" }).first();
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: back to desktop, delete the transaction then the position ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    await page.getByRole("button", { name: "Transaction actions" }).first().click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();

    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);
