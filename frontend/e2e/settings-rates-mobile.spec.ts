import { test, expect } from "@playwright/test";
import { pickMonth } from "./monthPicker";
import { deleteFxSeedAccount, seedUsdAccount } from "./fxSeed";

// Settings rate-table mobile divergence (#511, ADR-0050 B3b "wide table →
// stacked cards"). The Exchange-rates and Inflation-rates subpages each split at
// the renderer: useIsMobile (768px) mounts stacked cards on phones and the wide
// table on desktop, both under the same `fx-rate-row` / `inflation-rate-row`
// testids. These @smoke specs assert the correct renderer mounts at each width
// and the ADR-0050 a11y floor holds on the card — the rate (the figure the user
// came for) reads with no horizontal page scroll and the delete control clears
// 44px. Deep write-flow assertions stay in the nightly suite.
//
// At <768px the shell collapses the sidebar (ADR-0025), so navigate by URL.

// covers: INV-PRESENTATION-08
test(
  "inflation rates mount the card renderer and hold the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    // --- Seed one monthly inflation figure (desktop width) ---
    await page.goto("/settings/inflation-rates");
    await page.getByTestId("inflation-new-rate").click();
    const inflationDialog = page.getByRole("dialog");
    await pickMonth(page, inflationDialog.getByTestId("inflation-month"), "2026-05");
    await inflationDialog.getByLabel("Annual %").fill("3.5");
    await inflationDialog.getByRole("button", { name: "Add rate" }).click();
    await expect(page.getByTestId("inflation-rate-table")).toBeVisible();
    await expect(page.getByTestId("inflation-rate-value")).toHaveText("3.50%");

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings/inflation-rates");
    await expect(page.getByTestId("inflation-rate-cards")).toBeVisible();
    await expect(page.getByTestId("inflation-rate-table")).toHaveCount(0);

    // Primary value reachable: the rate reads on the card, within the viewport.
    const value = page.getByTestId("inflation-rate-value").first();
    await expect(value).toBeVisible();
    await expect(value).toBeInViewport();

    // No horizontal page scroll — the stacked card fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the card's ⋯ menu trigger clears 44px.
    const actions = page
      .getByTestId("inflation-rate-row")
      .getByRole("button", { name: "Inflation rate actions" });
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: delete the seeded figure via the mobile card menu ---
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await expect(page.getByTestId("inflation-rate-row")).toHaveCount(0);
  },
);

// covers: INV-PRESENTATION-08
test(
  "exchange rates mount the card renderer and hold the mobile a11y floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    // FX rates only surface once multi-currency is on — enable it first. The
    // checkbox is controlled by server state (it flips only after the settings
    // PATCH round-trips and the session query refetches), so click and await the
    // write rather than Playwright's check() which asserts the flip synchronously.
    await page.goto("/settings");
    const multi = page.getByTestId("multi-currency-toggle");
    if (!(await multi.isChecked())) {
      const saved = page.waitForResponse(
        (r) =>
          r.url().includes("/api/household/settings") && r.request().method() === "PATCH" && r.ok(),
      );
      await multi.click();
      await saved;
      await expect(multi).toBeChecked();
    }

    // --- Seed one monthly FX rate (desktop width). The currency select offers
    // only currencies the household's items use (#860), so seed a USD item
    // first and remove it before multi-currency goes back off. ---
    const usdAccountId = await seedUsdAccount(page);
    await page.goto("/settings/fx-rates");
    await page.getByTestId("fx-new-rate").click();
    const fxDialog = page.getByRole("dialog");
    await pickMonth(page, fxDialog.getByTestId("fx-month"), "2026-05");
    await fxDialog.getByLabel("Currency").selectOption("USD");
    // The add form's live hint spells the direction + counterpart as the code is
    // typed, so the foreign-only field isn't a clueless single input.
    await expect(page.getByTestId("fx-rate-hint")).toHaveText(/^1 USD = \? /);
    await fxDialog.getByLabel("Rate").fill("16250");
    await expect(page.getByTestId("fx-rate-hint")).toHaveText(/^1 USD = 16,250 /);
    await fxDialog.getByRole("button", { name: "Add rate" }).click();
    await expect(page.getByTestId("fx-rate-table")).toBeVisible();
    await expect(page.getByTestId("fx-rate-value")).toHaveText("16,250");
    // The currency column names the pair + direction, not just the foreign code.
    await expect(page.getByText(/USD →/)).toBeVisible();

    // --- Phone width: the renderer flips from table to cards ---
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto("/settings/fx-rates");
    await expect(page.getByTestId("fx-rate-cards")).toBeVisible();
    await expect(page.getByTestId("fx-rate-table")).toHaveCount(0);

    // Primary value reachable: the rate reads on the card as an equation (bound
    // to the reporting currency, so it can't misread as "16250 USD"), in view.
    const value = page.getByTestId("fx-rate-value").first();
    await expect(value).toBeVisible();
    await expect(value).toHaveText(/^1 USD = 16,250 /);
    await expect(value).toBeInViewport();

    // No horizontal page scroll — the stacked card fits the phone width.
    const overflow = await page.evaluate(() => {
      const el = document.scrollingElement ?? document.documentElement;
      return el.scrollWidth - el.clientWidth;
    });
    expect(overflow).toBeLessThanOrEqual(1);

    // Tap-target floor: the card's ⋯ menu trigger clears 44px.
    const actions = page
      .getByTestId("fx-rate-row")
      .getByRole("button", { name: "Exchange rate actions" });
    const box = await actions.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);

    // --- Cleanup: delete the seeded rate, then turn multi-currency back off ---
    await actions.click();
    await page.getByRole("menuitem", { name: "Delete" }).click();
    await expect(page.getByTestId("fx-rate-row")).toHaveCount(0);
    await page.setViewportSize({ width: 1280, height: 900 });
    await deleteFxSeedAccount(page, usdAccountId);
    await page.goto("/settings");
    const multiOff = page.getByTestId("multi-currency-toggle");
    const restored = page.waitForResponse(
      (r) =>
        r.url().includes("/api/household/settings") && r.request().method() === "PATCH" && r.ok(),
    );
    await multiOff.click();
    await restored;
    await expect(multiOff).not.toBeChecked();
  },
);
