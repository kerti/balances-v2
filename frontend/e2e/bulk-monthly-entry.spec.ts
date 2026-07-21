import { test, expect } from "@playwright/test";

// Bulk monthly-entry for the Asset group through the real UI + backend
// (ADR-0046): from Assets home, open "Enter this month", find a freshly-created
// bank account listed with no carried-forward value, type an amount, save all,
// then re-open the same month and confirm the row now warns it will overwrite —
// proving the batch persisted and the carry-forward surfacing flips to the
// same-month upsert state. Self-cleaning: deletes the parent account (taking its
// snapshot with it), leaving the seed's empty bank-account list. See ADR-0024.
// covers: INV-JOURNEYS-05
test(
  "asset bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E bulk account ${Date.now()}`;

    await page.goto("/assets/bank-accounts");

    // --- Create the parent bank account (no snapshots yet) ---
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    await acctDialog.getByLabel("Account number").fill("1234567890");
    await acctDialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(account) })).toBeVisible();

    // --- Open the bulk monthly-entry view from Assets home ---
    await page.goto("/assets");
    await expect(page.getByTestId("assets-home")).toBeVisible();
    await page.getByTestId("assets-enter-month").click();
    await expect(page.getByText("Enter this month's balances")).toBeVisible();

    // The new account is listed with no history and nothing counted as changed.
    const row = page.locator("li").filter({ hasText: account });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("asset-entry-dirty-count")).toHaveText("0 changed");

    // --- Type a value: the row goes dirty and the change count ticks up ---
    await row.getByRole("textbox").fill("12500000");
    await expect(page.getByTestId("asset-entry-dirty-count")).toHaveText("1 changed");

    // The statement date is left as the screen's seeded browser-local "today":
    // the backend future-date guard tolerates the max forward timezone offset,
    // so a UTC+ tester running pre-dawn no longer 400s (#426).

    // --- Save all: a successful batch redirects back to Assets home ---
    await page.getByTestId("asset-entry-save").click();
    await expect(page).toHaveURL(/\/assets$/);
    await expect(page.getByTestId("assets-home")).toBeVisible();

    // --- Re-open the same month: the write persisted, so the row now carries
    //     forward this month's own value and warns the next save overwrites it. ---
    await page.getByTestId("assets-enter-month").click();
    const savedRow = page.locator("li").filter({ hasText: account });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByRole("textbox")).toHaveValue("12500000");

    // --- Cleanup: delete the parent account (returns to the empty list) ---
    await page.goto("/assets/bank-accounts");
    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    const acctConfirm = page.getByRole("alertdialog");
    await acctConfirm.getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(account)).toHaveCount(0);
  },
);

// The same journey for the Liability group (#422): the mode generalises to the
// amount-only groups. Launched from the Liabilities Home header rather than an
// aggregate, and the group is grouped-by-subtype like Assets.
// covers: INV-JOURNEYS-05
test(
  "liability bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E bulk liability ${Date.now()}`;

    await page.goto("/liabilities/personal");
    await page.getByRole("button", { name: "New liability" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Display name").fill(name);
    await dialog.getByLabel("Counterparty").fill("E2E Counterparty");
    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    // Launch from the Liabilities Home header.
    await page.goto("/liabilities");
    await expect(page.getByTestId("liabilities-home")).toBeVisible();
    await page.getByTestId("liabilities-enter-month").click();
    await expect(page.getByText("Enter this month's balances")).toBeVisible();

    const row = page.locator("li").filter({ hasText: name });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("liability-entry-dirty-count")).toHaveText("0 changed");

    await row.getByRole("textbox").fill("250000000");
    await expect(page.getByTestId("liability-entry-dirty-count")).toHaveText("1 changed");
    // Seeded browser-local statement date submits as-is (tz-tolerant, #426).

    await page.getByTestId("liability-entry-save").click();
    await expect(page).toHaveURL(/\/liabilities$/);
    await expect(page.getByTestId("liabilities-home")).toBeVisible();

    await page.getByTestId("liabilities-enter-month").click();
    const savedRow = page.locator("li").filter({ hasText: name });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByRole("textbox")).toHaveValue("250000000");

    // Cleanup: delete via the list row actions menu.
    await page.goto("/liabilities/personal");
    const listRow = page.getByRole("row", { name: new RegExp(name) });
    await listRow.getByRole("button", { name: "Liability actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);

// The same journey for the Receivable group (#422) — the flat-group case: no
// Home screen, so entry is launched from the list toolbar, and the entry view
// renders one ungrouped list.
// covers: INV-JOURNEYS-05
test(
  "receivable bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E bulk receivable ${Date.now()}`;

    await page.goto("/receivables");
    await page.getByRole("button", { name: "New receivable" }).first().click();
    const dialog = page.getByRole("dialog");
    await dialog.getByLabel("Display name").fill(name);
    await dialog.getByLabel("Counterparty").fill("E2E Counterparty");
    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    // Launch from the list toolbar (flat group has no Home).
    await page.getByTestId("receivables-enter-month").click();
    await expect(page.getByText("Enter this month's balances")).toBeVisible();

    const row = page.locator("li").filter({ hasText: name });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("receivable-entry-dirty-count")).toHaveText("0 changed");

    await row.getByRole("textbox").fill("4500000");
    await expect(page.getByTestId("receivable-entry-dirty-count")).toHaveText("1 changed");
    // Seeded browser-local statement date submits as-is (tz-tolerant, #426).

    await page.getByTestId("receivable-entry-save").click();
    await expect(page).toHaveURL(/\/receivables$/);
    await expect(page.getByRole("button", { name: "New receivable" }).first()).toBeVisible();

    await page.getByTestId("receivables-enter-month").click();
    const savedRow = page.locator("li").filter({ hasText: name });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByRole("textbox")).toHaveValue("4500000");

    // Cleanup: delete via the list row actions menu.
    await page.goto("/receivables");
    const listRow = page.getByRole("row", { name: new RegExp(name) });
    await listRow.getByRole("button", { name: "Receivable actions" }).click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);

// The same journey for the Investment group's qty×price shape (#423): the mode
// extends past the amount-only groups to Stock/MutualFund/Gold, where a row has
// two tab-stops (quantity, price per unit) and the value is computed. Launched
// from the Investments Home header. Proves the two-field row goes dirty, saves,
// and re-opens carrying the same factors forward with the overwrite warning.
// covers: INV-JOURNEYS-05
test(
  "investment qty×price bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E bulk stock ${Date.now()}`;

    await page.goto("/investments/stocks");
    await page.getByRole("button", { name: "New stock" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("New stock position")).toBeVisible();
    await dialog.getByLabel("Display name").fill(name);
    await dialog.getByLabel("Ticker").fill("E2EB");
    await dialog.getByLabel("Exchange").fill("IDX");
    await dialog.getByLabel("Risk profile").selectOption("medium");
    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    // Launch the qty×price price-entry view from the Investments Home header.
    await page.goto("/investments");
    await expect(page.getByTestId("investments-home")).toBeVisible();
    await page.getByTestId("investments-enter-prices").click();
    await expect(page.getByText("Enter this month's prices")).toBeVisible();

    // The new stock is listed with no history and nothing counted as changed.
    const row = page.locator("li").filter({ hasText: name });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("investment-entry-dirty-count")).toHaveText("0 changed");

    // Typing only the quantity leaves the pair incomplete — still not dirty.
    await row.getByLabel("Quantity").fill("100");
    await expect(page.getByTestId("investment-entry-dirty-count")).toHaveText("0 changed");
    // Add the price: the pair completes, the row goes dirty, value is computed.
    await row.getByLabel("Price per unit").fill("8500");
    await expect(page.getByTestId("investment-entry-dirty-count")).toHaveText("1 changed");

    // Seeded browser-local statement date submits as-is (tz-tolerant, #426).

    await page.getByTestId("investment-entry-save").click();
    await expect(page).toHaveURL(/\/investments$/);
    await expect(page.getByTestId("investments-home")).toBeVisible();

    // Re-open the same month: the write persisted, so the row carries forward its
    // own quantity + price and warns the next save overwrites it.
    await page.getByTestId("investments-enter-prices").click();
    const savedRow = page.locator("li").filter({ hasText: name });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByLabel("Quantity")).toHaveValue("100");
    await expect(savedRow.getByLabel("Price per unit")).toHaveValue("8500");

    // Cleanup: delete the parent stock from its detail page.
    await page.goto("/investments/stocks");
    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);

test(
  "investment accrued bulk monthly-entry: list → save → re-open shows overwrite",
  { tag: "@smoke" },
  async ({ page }) => {
    const name = `E2E bulk bond ${Date.now()}`;

    await page.goto("/investments/bonds");
    await page.getByRole("button", { name: "New bond" }).first().click();
    const dialog = page.getByRole("dialog");
    await expect(dialog.getByText("New bond position")).toBeVisible();
    await dialog.getByLabel("Display name").fill(name);
    await dialog.getByLabel("Issuer").fill("E2E Treasury");
    await dialog.getByLabel("Face value").fill("1000000");
    await dialog.getByLabel("Coupon rate (% per year)").fill("6.5");
    await dialog.getByLabel("Maturity date").fill("2030-01-01");
    await dialog.getByLabel("Placement date").fill("2024-01-01");
    await dialog.getByLabel("Risk profile").selectOption("medium");
    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(name) })).toBeVisible();

    // Launch the accrued value-entry view from the Investments Home header.
    await page.goto("/investments");
    await expect(page.getByTestId("investments-home")).toBeVisible();
    await page.getByTestId("investments-enter-accrued").click();
    await expect(page.getByText("Enter this month's bond & deposit values")).toBeVisible();

    // The new bond is listed with no history and nothing counted as changed.
    const row = page.locator("li").filter({ hasText: name });
    await expect(row.getByText("No previous value")).toBeVisible();
    await expect(page.getByTestId("investment-accrued-entry-dirty-count")).toHaveText("0 changed");

    // A govt-primary bond defaults to pays-out, so accrued seeds to 0; typing
    // only the total value already completes the row (accrued has its default).
    await row.getByLabel("Total value").fill("1010000");
    await expect(page.getByTestId("investment-accrued-entry-dirty-count")).toHaveText("1 changed");

    // Seeded browser-local statement date submits as-is (tz-tolerant, #426).

    await page.getByTestId("investment-accrued-entry-save").click();
    await expect(page).toHaveURL(/\/investments$/);
    await expect(page.getByTestId("investments-home")).toBeVisible();

    // Re-open the same month: the write persisted, so the row carries forward its
    // own total value + accrued and warns the next save overwrites it.
    await page.getByTestId("investments-enter-accrued").click();
    const savedRow = page.locator("li").filter({ hasText: name });
    await expect(savedRow.getByText("Will overwrite")).toBeVisible();
    await expect(savedRow.getByLabel("Total value")).toHaveValue("1010000");
    await expect(savedRow.getByLabel("Accrued")).toHaveValue("0");

    // Cleanup: delete the parent bond from its detail page.
    await page.goto("/investments/bonds");
    await page
      .getByRole("row", { name: new RegExp(name) })
      .getByText(name)
      .click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(name)).toHaveCount(0);
  },
);

// ADR-0050 S1 (#502): the amount-only entry row diverges its mobile layout via
// the runtime pick-one renderer — one tree in the DOM, same testids at both
// widths. This asserts the CORRECT renderer mounts per viewport and the value
// stays reachable: at <768px the stacked EntryRowMobile (full-width h-11 input,
// the ≥44px tap floor), at ≥768px the cramped EntryRowDesktop (w-36). Deep
// per-shape behaviour is already covered by the amount-only journeys above.
// covers: INV-JOURNEYS-05
test(
  "asset entry row diverges renderer at the 768px boundary",
  { tag: "@smoke" },
  async ({ page }) => {
    const account = `E2E divergence account ${Date.now()}`;

    await page.goto("/assets/bank-accounts");
    await page.getByRole("button", { name: "New bank account" }).first().click();
    const acctDialog = page.getByRole("dialog");
    await acctDialog.getByLabel("Display name").fill(account);
    await acctDialog.getByLabel("Bank name").fill("E2E Bank");
    await acctDialog.getByLabel("Account number").fill("1234567890");
    await acctDialog.getByRole("button", { name: "Create" }).click();
    await expect(page.getByRole("row", { name: new RegExp(account) })).toBeVisible();

    // --- Mobile width: the stacked renderer mounts, value input is a full-width
    //     ≥44px target, and no horizontal scroll is needed to reach it. ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/assets");
    await page.getByTestId("assets-enter-month").click();
    await expect(page.getByText("Enter this month's balances")).toBeVisible();
    const mobileRow = page.locator("li").filter({ hasText: account });
    const mobileInput = mobileRow.getByRole("textbox");
    await expect(mobileInput).toHaveClass(/h-11/);
    await expect(mobileInput).toBeInViewport();
    const boundingBox = await mobileInput.boundingBox();
    expect(boundingBox!.height).toBeGreaterThanOrEqual(44);

    // --- Desktop width: reload swaps to the cramped renderer (w-36 input). ---
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.reload();
    await page.getByTestId("assets-enter-month").click();
    const desktopRow = page.locator("li").filter({ hasText: account });
    await expect(desktopRow.getByRole("textbox")).toHaveClass(/w-36/);

    // --- Cleanup ---
    await page.goto("/assets/bank-accounts");
    await page
      .getByRole("row", { name: new RegExp(account) })
      .getByText(account)
      .click();
    await expect(page.getByRole("heading", { level: 1, name: account })).toBeVisible();
    await page.getByRole("button", { name: "Delete" }).click();
    await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
    await expect(page.getByText(account)).toHaveCount(0);
  },
);
