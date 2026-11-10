import { test, expect, type Page } from "@playwright/test";

import { pickMonth } from "./monthPicker";

// InfoHint on a phone (#841). The old "ⓘ" glyph and explanatory `title`
// tooltips never showed on touch; InfoHint opens a popover on tap. This asserts
// at 390px that the dashboard's non-cash hint and a risk badge both open by tap
// and that their triggers clear the ≥44px floor, and that the snapshot dialog's
// month picker (replacing the native month input) drives the form.
//
// Seeds through the founder session (page.request carries the cookie) and tears
// down afterwards: a property with two consecutive monthly snapshots (previous and current month, so the latest report month carries the change) (so the
// latest month carries a non-cash asset value change) and a stock for the badge.
//
// covers: INV-PRESENTATION-08
test(
  "InfoHints open by tap and clear the 44px floor at 390px",
  { tag: "@smoke" },
  async ({ page }) => {
    const stamp = Date.now();
    const propertyName = `E2E hint property ${stamp}`;
    const stockName = `E2E hint stock ${stamp}`;
    const now = new Date();
    const monthsAgo = (n: number) => {
      const d = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - n, 1));
      return `${d.getUTCFullYear()}-${String(d.getUTCMonth() + 1).padStart(2, "0")}`;
    };

    const propRes = await page.request.post("/api/properties", {
      data: {
        display_name: propertyName,
        description: null,
        ownership_type: "joint",
        sole_owner_user_id: null,
        native_currency: "IDR",
        property_type: "house",
        address: null,
        acquisition_date: null,
        acquisition_cost: null,
        annual_appreciation_rate: null,
        entry_type: "newly_tracked",
      },
    });
    expect(propRes.ok(), "seed property").toBeTruthy();
    const propertyId = ((await propRes.json()) as { asset: { id: string } }).asset.id;
    for (const [month, amount] of [
      [monthsAgo(1), "500000000"],
      [monthsAgo(0), "450000000"],
    ]) {
      const res = await page.request.post(`/api/assets/${propertyId}/snapshots`, {
        data: { year_month: month, amount, currency: "IDR", as_of_date: null, description: null },
      });
      expect(res.ok(), `seed snapshot ${month}`).toBeTruthy();
    }

    const stockRes = await page.request.post("/api/investments/stocks", {
      data: {
        display_name: stockName,
        description: null,
        ownership_type: "joint",
        sole_owner_user_id: null,
        risk_profile: "medium",
        native_currency: "IDR",
        ticker: "TST",
        exchange: "TEST",
        entry_type: "newly_tracked",
      },
    });
    expect(stockRes.ok(), "seed stock").toBeTruthy();
    const stockId = ((await stockRes.json()) as { investment: { id: string } }).investment.id;

    try {
      await page.setViewportSize({ width: 390, height: 844 });

      // --- Dashboard: the non-cash hint opens by tap ---
      await page.goto("/");
      await expect(page.getByRole("heading", { level: 1, name: "Net worth" })).toBeVisible();
      const nonCash = page.getByTestId("dashboard-hint-non-cash");
      await nonCash.scrollIntoViewIfNeeded();
      await expect(nonCash).toBeVisible();
      const box = await nonCash.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.width).toBeGreaterThanOrEqual(44);
      expect(box!.height).toBeGreaterThanOrEqual(44);
      await nonCash.click();
      await expect(page.getByTestId("dashboard-hint-non-cash-content")).toContainText("Non-cash");

      // --- Risk badge label opens by tap ---
      await page.goto("/investments/stocks");
      const badge = page.getByTestId("risk-profile-hint-medium").first();
      await expect(badge).toBeVisible();
      await badge.click();
      await expect(page.getByTestId("risk-profile-hint-medium-content")).toHaveText("Medium risk");

      // --- Month picker replaces the native month input in the snapshot dialog ---
      await page.goto(`/assets/properties/${propertyId}`);
      await page.getByRole("button", { name: "Record snapshot" }).click();
      const dialog = page.getByRole("dialog");
      await pickMonth(page, dialog.getByTestId("year_month"), monthsAgo(3));
      await expect(page.getByTestId("month-picker-content")).toHaveCount(0);
      await expect(dialog.getByTestId("year_month")).toContainText(monthsAgo(3).slice(0, 4));
    } finally {
      await cleanup(page, propertyId, stockId);
    }
  },
);

async function cleanup(page: Page, propertyId: string, stockId: string) {
  await page.request.delete(`/api/properties/${propertyId}`);
  await page.request.delete(`/api/investments/stocks/${stockId}`);
}
