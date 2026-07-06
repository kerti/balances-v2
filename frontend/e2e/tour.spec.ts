import { test, expect, type Page } from "@playwright/test";

// Built-in instruction-manual tours through the real UI (issue #23, #26). The
// `help-tour` button on every position detail screen launches a driver.js
// guided tour; HelpTourButton prunes steps whose `data-testid` anchor isn't
// rendered this visit (the chart needs ≥2 snapshots) and feeds driver.js the
// translated chrome from `common:tour.*`. driver.js renders its popover in a
// portal with role="dialog" — we assert on its rendered text and on the
// `driver-active-element` class it stamps onto the highlighted anchor, never on
// a fixed DOM position (see e2e/README.md). All specs self-clean back to the
// seed's empty lists. See ADR-0024.

type Step = { id: string; title: string };

// Bank-account variant: 5 steps when ≥2 snapshots make the chart render.
const BANK_STEPS: Step[] = [
  { id: "tour-overview", title: "Your account at a glance" },
  { id: "tour-actions", title: "Manage this position" },
  { id: "tour-details", title: "The account's details" },
  { id: "tour-chart", title: "Balance over time" },
  { id: "tour-snapshots", title: "Monthly balances" },
];

// Investment (bond) variant: 7 steps, adding the headline + transactions anchors.
const BOND_STEPS: Step[] = [
  { id: "tour-overview", title: "Your bond at a glance" },
  { id: "investment-headline", title: "Cost and profit/loss" },
  { id: "tour-actions", title: "Manage this position" },
  { id: "tour-details", title: "The bond's terms" },
  { id: "tour-chart", title: "Value over time" },
  { id: "tour-snapshots", title: "Monthly values" },
  { id: "tour-transactions", title: "Payments, trades, and maturity" },
];

async function createBankAccount(page: Page, name: string) {
  await page.goto("/assets/bank-accounts");
  await page.getByRole("button", { name: "New bank account" }).first().click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Display name").fill(name);
  await dialog.getByLabel("Bank name").fill("E2E Bank");
  await dialog.getByLabel("Account number").fill("1234567890");
  await dialog.getByRole("button", { name: "Create" }).click();
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
}

async function createBond(page: Page, name: string) {
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
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
}

// Amount-only snapshot (bank account). Two distinct months are needed to clear
// the chart's `snapshots.length >= 2` gate; the month is keyed per position.
async function addBankSnapshot(page: Page, month: string, amount: string) {
  await page.getByRole("button", { name: "New" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Record monthly snapshot")).toBeVisible();
  await dialog.getByLabel("Month").fill(month);
  await dialog.getByLabel("Amount (IDR)").fill(amount);
  await dialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}

// Accrued-interest snapshot (bond).
async function addBondSnapshot(page: Page, month: string, total: string) {
  await page.getByRole("button", { name: "New" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Record monthly snapshot")).toBeVisible();
  await dialog.getByLabel("Month").fill(month);
  await dialog.getByLabel("Total value (IDR)").fill(total);
  await dialog.getByLabel("Accrued (IDR)").fill("10000");
  await dialog.getByRole("button", { name: "Save snapshot" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
}

async function deletePosition(page: Page) {
  await page.getByRole("button", { name: "Delete" }).first().click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete" }).click();
}

// Assert the popover for step `index` (0-based) of a `total`-step tour: the
// title renders, the progress text reads "current of total", and driver.js
// stamped `driver-active-element` onto the step's anchor — i.e. the popover
// attached to the right element.
async function expectStep(page: Page, index: number, total: number, step: Step) {
  const popover = page.getByRole("dialog");
  await expect(popover.getByText(step.title)).toBeVisible();
  await expect(popover.getByText(`${index + 1} of ${total}`)).toBeVisible();
  await expect(page.getByTestId(step.id)).toHaveClass(/driver-active-element/);
}

test("non-investment tour: launch, navigate forward/back, done (5 steps)", async ({ page }) => {
  const name = `E2E tour account ${Date.now()}`;
  await createBankAccount(page, name);
  await addBankSnapshot(page, "2024-01", "10000000");
  await addBankSnapshot(page, "2024-02", "12000000");

  // The chart renders now (2 snapshots), so all 5 steps are present.
  const popover = page.getByRole("dialog");
  await expect(page.getByTestId("help-tour")).toBeVisible();
  await page.getByTestId("help-tour").click();

  // Launch lands on step 1 with its title visible.
  await expectStep(page, 0, 5, BANK_STEPS[0]);

  // Next advances; Back returns to the prior step.
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 1, 5, BANK_STEPS[1]);
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 2, 5, BANK_STEPS[2]);
  await popover.getByRole("button", { name: "Back" }).click();
  await expectStep(page, 1, 5, BANK_STEPS[1]);

  // Walk to the last step; each anchor (incl. the chart) attaches correctly.
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 2, 5, BANK_STEPS[2]);
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 3, 5, BANK_STEPS[3]);
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 4, 5, BANK_STEPS[4]);

  // The final step swaps Next → Done; clicking it closes the overlay.
  await popover.getByRole("button", { name: "Done" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await deletePosition(page);
  await expect(page.getByText(name)).toHaveCount(0);
});

test("investment tour: 7 steps incl. headline + transactions", async ({ page }) => {
  const name = `E2E tour bond ${Date.now()}`;
  await createBond(page, name);
  await addBondSnapshot(page, "2024-01", "1005000");
  await addBondSnapshot(page, "2024-02", "1010000");

  const popover = page.getByRole("dialog");
  await page.getByTestId("help-tour").click();

  for (let i = 0; i < BOND_STEPS.length; i++) {
    await expectStep(page, i, BOND_STEPS.length, BOND_STEPS[i]);
    if (i < BOND_STEPS.length - 1) {
      await popover.getByRole("button", { name: "Next" }).click();
    }
  }
  await popover.getByRole("button", { name: "Done" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await deletePosition(page);
  await expect(page.getByText(name)).toHaveCount(0);
});

test("chart step is pruned when the position has fewer than two snapshots", async ({ page }) => {
  const name = `E2E tour nochart ${Date.now()}`;
  await createBankAccount(page, name);
  // No snapshots: the chart anchor never renders, so the tour drops to 4 steps.
  await expect(page.getByTestId("tour-chart")).toHaveCount(0);

  const popover = page.getByRole("dialog");
  await page.getByTestId("help-tour").click();

  const pruned: Step[] = [
    BANK_STEPS[0], // overview
    BANK_STEPS[1], // actions
    BANK_STEPS[2], // details
    BANK_STEPS[4], // snapshots — chart skipped
  ];
  for (let i = 0; i < pruned.length; i++) {
    await expectStep(page, i, pruned.length, pruned[i]);
    // The chart step's title never appears in the pruned run. Exact match: the
    // overview body copy also contains the phrase "balance over time".
    await expect(popover.getByText("Balance over time", { exact: true })).toHaveCount(0);
    if (i < pruned.length - 1) {
      await popover.getByRole("button", { name: "Next" }).click();
    }
  }
  await popover.getByRole("button", { name: "Done" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await deletePosition(page);
  await expect(page.getByText(name)).toHaveCount(0);
});

test("closed position: the actions step still anchors to the header group", async ({ page }) => {
  const name = `E2E tour closed ${Date.now()}`;
  await createBankAccount(page, name);

  // Close the position. The per-card New/Import buttons hide, but the header
  // action group (tour-actions) is always rendered, so its step survives.
  await page.getByRole("button", { name: "Close", exact: true }).click();
  const closeDialog = page.getByRole("dialog");
  await expect(closeDialog.getByText("Close position")).toBeVisible();
  await closeDialog.getByLabel("Status").selectOption("closed");
  await closeDialog.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("status-badge")).toHaveText("Closed");

  const popover = page.getByRole("dialog");
  await page.getByTestId("help-tour").click();
  // 4 steps (no snapshots → chart pruned), header action group still present.
  await expectStep(page, 0, 4, BANK_STEPS[0]);
  await popover.getByRole("button", { name: "Next" }).click();
  await expectStep(page, 1, 4, BANK_STEPS[1]); // tour-actions, anchored to header

  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  await deletePosition(page);
  await expect(page.getByText(name)).toHaveCount(0);
});

test("tour copy follows the Settings language (EN default → ID)", async ({ page }) => {
  const name = `E2E tour locale ${Date.now()}`;
  await createBankAccount(page, name);

  // Default locale is en-GB (the e2e pin): chrome + body render in English.
  const popover = page.getByRole("dialog");
  await page.getByTestId("help-tour").click();
  await expect(popover.getByText("Your account at a glance")).toBeVisible();
  await expect(popover.getByRole("button", { name: "Next" })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // Switch to Indonesian via the Settings dropdown (never via the seed). The
  // choice is optimistic + a PATCH /api/me; wait for that to land before the
  // reload so useLocaleReconcile reads id-ID from /me and doesn't revert to EN.
  await page.goto("/settings");
  const idPatch = page.waitForResponse(
    (r) => r.url().includes("/api/me") && r.request().method() === "PATCH",
  );
  await page.getByTestId("settings-language-select").selectOption("id-ID");
  await idPatch;

  await page.goto("/assets/bank-accounts");
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();

  // Help button + tour now read Indonesian: chrome (common:tour.*) and a body string.
  await page.getByTestId("help-tour").click();
  await expect(popover.getByText("Sekilas tentang rekening Anda")).toBeVisible();
  await expect(
    popover.getByText(
      "Lacak saldo rekening ini dari waktu ke waktu — catat tiap bulan agar kekayaan bersih Anda selalu terkini.",
    ),
  ).toBeVisible();
  await expect(popover.getByText("1 dari 4")).toBeVisible();
  await popover.getByRole("button", { name: "Lanjut" }).click();
  await expect(popover.getByText("Kelola posisi ini")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);

  // Restore the en-GB pin so later specs see English (self-cleaning). Wait for
  // the PATCH so the DB + /me are back to en-GB before the next spec boots.
  await page.goto("/settings");
  const enPatch = page.waitForResponse(
    (r) => r.url().includes("/api/me") && r.request().method() === "PATCH",
  );
  await page.getByTestId("settings-language-select").selectOption("en-GB");
  await enPatch;

  await page.goto("/assets/bank-accounts");
  await page
    .getByRole("row", { name: new RegExp(name) })
    .getByText(name)
    .click();
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
  await deletePosition(page);
  await expect(page.getByText(name)).toHaveCount(0);
});
