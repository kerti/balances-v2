import { expect, type Page } from "@playwright/test";

// The Exchange-rates New form offers only the currencies the household's items
// are recorded in (#860), so a spec that adds a USD rate through the UI first
// needs a USD item. Seeds a USD bank account with one snapshot over the API and
// returns its id; delete it with `deleteFxSeedAccount` before turning
// multi-currency off (the toggle refuses while a foreign position exists).
export async function seedUsdAccount(page: Page): Promise<string> {
  const res = await page.request.post("/api/bank-accounts", {
    data: {
      display_name: `E2E fx usd account ${Date.now()}`,
      description: null,
      ownership_type: "joint",
      sole_owner_user_id: null,
      native_currency: "USD",
      bank_name: "E2E Bank",
      account_number: "",
      account_type: "savings",
      entry_type: "newly_tracked",
    },
  });
  expect(res.ok(), "seed USD account").toBeTruthy();
  const id = ((await res.json()) as { asset: { id: string } }).asset.id;
  const now = new Date();
  const month = `${now.getUTCFullYear()}-${String(now.getUTCMonth() + 1).padStart(2, "0")}`;
  const snap = await page.request.post(`/api/assets/${id}/snapshots`, {
    data: {
      year_month: month,
      amount: "1000",
      currency: "USD",
      as_of_date: null,
      description: null,
    },
  });
  expect(snap.ok(), "seed USD snapshot").toBeTruthy();
  return id;
}

export async function deleteFxSeedAccount(page: Page, id: string): Promise<void> {
  const res = await page.request.delete(`/api/bank-accounts/${id}`);
  expect(res.ok(), "delete USD account").toBeTruthy();
}
