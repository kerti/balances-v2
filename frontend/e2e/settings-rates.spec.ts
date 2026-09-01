import { test, expect, type Page } from "@playwright/test";

// Desktop write-flow coverage for the Settings ▸ Inflation Rates and ▸ Exchange
// Rates subpages: inline rate editing and the 12-per-page pagination. The mobile
// renderer + a11y floor is the per-PR @smoke concern (settings-rates-mobile.spec)
// — these deeper flows live in the full/nightly suite, matching tags.spec.ts.
//
// Rows are seeded and torn down through the shared founder session (page.request
// carries Alice's cookie), so the tests don't drive the add-form 13× and leave
// the inflation_rates / fx_rates tables empty again afterwards. The e2e seed
// creates no rates, so the row counts these assert on are exact.

async function seedInflation(page: Page, yearMonth: string, rate: string): Promise<string> {
  const res = await page.request.post("/api/inflation-rates", {
    data: { year_month: yearMonth, rate },
  });
  expect(res.ok(), `seed inflation ${yearMonth}`).toBeTruthy();
  return ((await res.json()) as { id: string }).id;
}

async function seedFx(
  page: Page,
  yearMonth: string,
  currency: string,
  rate: string,
): Promise<string> {
  const res = await page.request.post("/api/fx-rates", {
    data: { year_month: yearMonth, currency, rate },
  });
  expect(res.ok(), `seed fx ${yearMonth}`).toBeTruthy();
  return ((await res.json()) as { id: string }).id;
}

// Twelve consecutive months + one more → 13 rows, i.e. two pages at PAGE_SIZE 12.
function thirteenMonths(): string[] {
  const months = Array.from({ length: 12 }, (_, m) => `2024-${String(m + 1).padStart(2, "0")}`);
  months.push("2025-01");
  return months;
}

// The FX card only mounts once multi-currency is on. The toggle is server-backed
// (it flips after the settings PATCH round-trips), so click and await the write
// rather than check(), which asserts the flip synchronously.
async function setMultiCurrency(page: Page, on: boolean): Promise<void> {
  await page.goto("/settings");
  const toggle = page.getByTestId("multi-currency-toggle");
  if ((await toggle.isChecked()) === on) return;
  const saved = page.waitForResponse(
    (r) =>
      r.url().includes("/api/household/settings") && r.request().method() === "PATCH" && r.ok(),
  );
  await toggle.click();
  await saved;
  await expect(toggle).toBeChecked({ checked: on });
}

test("inflation rates: edit a row in place, display formats to 2dp", async ({ page }) => {
  const id = await seedInflation(page, "2026-05", "3.5");
  try {
    await page.goto("/settings/inflation-rates");
    // Read display is a fixed 2dp percentage, not the raw stored "3.5".
    await expect(page.getByTestId("inflation-rate-value")).toHaveText("3.50%");

    const row = page.getByTestId("inflation-rate-row");
    await row.getByRole("button", { name: "Edit", exact: true }).click();

    // The rate swaps to an input pre-filled with the raw value (scoped to the
    // row — the add form shares the "Annual %" label).
    const input = row.getByLabel("Annual %");
    await expect(input).toHaveValue("3.5");
    await input.fill("4.25");

    const saved = page.waitForResponse(
      (r) =>
        r.url().includes("/api/inflation-rates/") && r.request().method() === "PATCH" && r.ok(),
    );
    await row.getByRole("button", { name: "Save", exact: true }).click();
    await saved;

    // Back to the read display, now showing the edited figure at 2dp.
    await expect(page.getByTestId("inflation-rate-value")).toHaveText("4.25%");
  } finally {
    await page.request.delete(`/api/inflation-rates/${id}`);
  }
});

test("inflation rates: paginate at 12 rows per page", async ({ page }) => {
  const ids: string[] = [];
  for (const m of thirteenMonths()) ids.push(await seedInflation(page, m, "1"));
  try {
    await page.goto("/settings/inflation-rates");
    await expect(page.getByTestId("inflation-rate-row")).toHaveCount(12);

    // Jump to page 2 → the single remaining row. First + last are always one tap.
    await page.getByRole("link", { name: "2", exact: true }).click();
    await expect(page.getByTestId("inflation-rate-row")).toHaveCount(1);
  } finally {
    for (const id of ids) await page.request.delete(`/api/inflation-rates/${id}`);
  }
});

test("exchange rates: edit a row in place", async ({ page }) => {
  await setMultiCurrency(page, true);
  const id = await seedFx(page, "2026-05", "USD", "16250");
  try {
    await page.goto("/settings/fx-rates");
    await expect(page.getByTestId("fx-rate-value")).toHaveText("16250");

    const row = page.getByTestId("fx-rate-row");
    await row.getByRole("button", { name: "Edit", exact: true }).click();

    // Scoped to the row — the add form shares the "Rate" label.
    const input = row.getByLabel("Rate");
    await expect(input).toHaveValue("16250");
    await input.fill("16400");

    const saved = page.waitForResponse(
      (r) => r.url().includes("/api/fx-rates/") && r.request().method() === "PATCH" && r.ok(),
    );
    await row.getByRole("button", { name: "Save", exact: true }).click();
    await saved;

    await expect(page.getByTestId("fx-rate-value")).toHaveText("16400");
  } finally {
    await page.request.delete(`/api/fx-rates/${id}`);
    await setMultiCurrency(page, false);
  }
});

test("exchange rates: paginate at 12 rows per page", async ({ page }) => {
  await setMultiCurrency(page, true);
  const ids: string[] = [];
  for (const m of thirteenMonths()) ids.push(await seedFx(page, m, "USD", "16000"));
  try {
    await page.goto("/settings/fx-rates");
    await expect(page.getByTestId("fx-rate-row")).toHaveCount(12);

    await page.getByRole("link", { name: "2", exact: true }).click();
    await expect(page.getByTestId("fx-rate-row")).toHaveCount(1);
  } finally {
    for (const id of ids) await page.request.delete(`/api/fx-rates/${id}`);
    await setMultiCurrency(page, false);
  }
});
