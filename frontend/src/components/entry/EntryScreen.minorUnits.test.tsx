// A bulk-entry prefill is shown rounded to the currency's minor units but saved
// as stored (INC-32, ADR-0056, #784): the display string is never the dirty-check
// baseline, so an untouched row writes nothing, and a quantity or price keeps
// every stored place.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import {
  assetEntryConfig,
  investmentAccruedEntryConfig,
  investmentEntryConfig,
} from "@/components/entry/groups";

const base = {
  subtype: "bank_account",
  ownership_type: "joint",
  sole_owner_user_id: null,
  carried_from: "2026-04",
};

// Stored values carry more places than the currency shows: IDR with 4, a USD
// figure with one, a JPY figure with a fraction, a zero, and a negative.
const assetList = {
  year_month: "2026-05",
  rows: [
    {
      ...base,
      asset_id: "a1",
      display_name: "BCA",
      currency: "IDR",
      prefill_amount: "1408210.1634",
    },
    {
      ...base,
      asset_id: "a2",
      display_name: "Mandiri",
      currency: "IDR",
      prefill_amount: "48056131.79",
    },
    { ...base, asset_id: "u1", display_name: "Wise", currency: "USD", prefill_amount: "100.5" },
    { ...base, asset_id: "j1", display_name: "Rakuten", currency: "JPY", prefill_amount: "1500.4" },
    { ...base, asset_id: "z1", display_name: "Empty", currency: "IDR", prefill_amount: "0" },
    {
      ...base,
      asset_id: "n1",
      display_name: "Overdraft",
      currency: "IDR",
      prefill_amount: "-250.1264",
    },
  ],
};

function useAssets() {
  const posts: Array<{ rows: Array<Record<string, string>> }> = [];
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(assetList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/assets/snapshots/bulk", async ({ request }) => {
      posts.push((await request.json()) as (typeof posts)[number]);
      return HttpResponse.json({ written: 1 });
    }),
  );
  return posts;
}

function renderScreen(config: Parameters<typeof EntryScreen>[0]["config"]) {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={config} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-10
it("shows a carried-forward amount at the currency's minor units", async () => {
  useAssets();
  renderScreen(assetEntryConfig);

  expect(await screen.findByTestId("asset-entry-amount-a1")).toHaveValue("1,408,210.16");
  expect(screen.getByTestId("asset-entry-amount-a2")).toHaveValue("48,056,131.79");
  expect(screen.getByTestId("asset-entry-amount-u1")).toHaveValue("100.5");
  expect(screen.getByTestId("asset-entry-amount-j1")).toHaveValue("1,500");
  expect(screen.getByTestId("asset-entry-amount-z1")).toHaveValue("0");
  expect(screen.getByTestId("asset-entry-amount-n1")).toHaveValue("-250.13");
});

// covers: INV-SNAPSHOTS-10
// covers: INV-SNAPSHOTS-06
it("an untouched row is never dirty and never in the save payload, even focused and left again", async () => {
  const posts = useAssets();
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  const a1 = await screen.findByTestId("asset-entry-amount-a1");
  // Shown rounded, stored with 4 places: not dirty, Save disabled.
  expect(a1).toHaveValue("1,408,210.16");
  expect(screen.getByTestId("asset-entry-dirty-count")).toHaveTextContent("0 changed");
  expect(screen.getByTestId("asset-entry-save")).toBeDisabled();

  // Tabbing through the field changes nothing either.
  await user.click(a1);
  await user.tab();
  expect(screen.getByTestId("asset-entry-dirty-count")).toHaveTextContent("0 changed");
  expect(screen.getByTestId("asset-entry-save")).toBeDisabled();
  expect(screen.queryByTestId("asset-entry-dirty-a1")).not.toBeInTheDocument();
  expect(posts).toHaveLength(0);
});

// covers: INV-SNAPSHOTS-10
// covers: INV-SNAPSHOTS-06
it("saving one edited row sends that row's typed canonical value and leaves the rounded rows out", async () => {
  const posts = useAssets();
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  const a2 = await screen.findByTestId("asset-entry-amount-a2");
  await user.clear(a2);
  await user.type(a2, "50000000.5");
  expect(screen.getByTestId("asset-entry-dirty-count")).toHaveTextContent("1 changed");
  await user.click(screen.getByTestId("asset-entry-save"));

  await waitFor(() => expect(posts).toHaveLength(1));
  // Only a2: a1 (stored 4dp, shown rounded) and the rest are untouched.
  expect(posts[0].rows).toHaveLength(1);
  expect(posts[0].rows[0]).toMatchObject({
    asset_id: "a2",
    amount: "50000000.5",
    currency: "IDR",
  });
});

// covers: INV-SNAPSHOTS-10
it("typing into a rounded prefill sends the canonical string of what is then shown", async () => {
  const posts = useAssets();
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  // "1,408,210.16" is shown; the field is at its 2-place limit, so the edit is a
  // replacement. Whatever ends up in the field goes out in canonical form.
  const a1 = await screen.findByTestId("asset-entry-amount-a1");
  await user.clear(a1);
  await user.type(a1, "1500000.25");
  await user.click(screen.getByTestId("asset-entry-save"));

  await waitFor(() => expect(posts).toHaveLength(1));
  expect(posts[0].rows[0]).toMatchObject({ asset_id: "a1", amount: "1500000.25" });
});

const qtyList = {
  year_month: "2026-05",
  rows: [
    {
      investment_id: "s1",
      display_name: "BBCA",
      currency: "IDR",
      subtype: "stock",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_quantity: "10.12345678",
      prefill_price: "1500.1234",
      carried_from: "2026-04",
    },
  ],
};

// covers: INV-SNAPSHOTS-10
it("never rounds a quantity or a price per unit to money's minor units", async () => {
  const posts: Array<{ rows: Array<Record<string, string>> }> = [];
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(qtyList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/bulk", async ({ request }) => {
      posts.push((await request.json()) as (typeof posts)[number]);
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen(investmentEntryConfig);
  const user = userEvent.setup();

  expect(await screen.findByTestId("investment-entry-quantity-s1")).toHaveValue("10.12345678");
  expect(screen.getByTestId("investment-entry-price-s1")).toHaveValue("1,500.1234");
  expect(screen.getByTestId("investment-entry-dirty-count")).toHaveTextContent("0 changed");

  // Editing the price alone sends the quantity as stored, all 8 places.
  const price = screen.getByTestId("investment-entry-price-s1");
  await user.clear(price);
  await user.type(price, "1600");
  await user.click(screen.getByTestId("investment-entry-save"));

  await waitFor(() => expect(posts).toHaveLength(1));
  expect(posts[0].rows[0]).toMatchObject({
    investment_id: "s1",
    quantity: "10.12345678",
    price_per_unit: "1600",
  });
});

const accruedList = {
  year_month: "2026-05",
  rows: [
    {
      investment_id: "b1",
      display_name: "FR Series Bond",
      currency: "IDR",
      subtype: "bond",
      ownership_type: "joint",
      sole_owner_user_id: null,
      coupon_disposition: "pays_out",
      prefill_amount: "50250000.1234",
      prefill_accrued_interest: "250000.5678",
      carried_from: "2026-04",
    },
  ],
};

// covers: INV-SNAPSHOTS-10
it("an accrued row shows both money fields rounded but sends the untouched one as stored", async () => {
  const posts: Array<{ rows: Array<Record<string, string>> }> = [];
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(accruedList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/accrued/bulk", async ({ request }) => {
      posts.push((await request.json()) as (typeof posts)[number]);
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen(investmentAccruedEntryConfig);
  const user = userEvent.setup();

  expect(await screen.findByTestId("investment-accrued-entry-amount-b1")).toHaveValue(
    "50,250,000.12",
  );
  const accrued = screen.getByTestId("investment-accrued-entry-accrued-b1");
  expect(accrued).toHaveValue("250,000.57");
  expect(screen.getByTestId("investment-accrued-entry-dirty-count")).toHaveTextContent("0 changed");

  await user.clear(accrued);
  await user.type(accrued, "300000");
  await user.click(screen.getByTestId("investment-accrued-entry-save"));

  await waitFor(() => expect(posts).toHaveLength(1));
  expect(posts[0].rows[0]).toMatchObject({
    investment_id: "b1",
    amount: "50250000.1234",
    accrued_interest: "300000",
  });
});
