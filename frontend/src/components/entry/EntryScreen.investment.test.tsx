// Conformance test for the Investment qty×price bulk monthly-entry view
// (ADR-0046, #423). The two-tab-stop twin of the amount-only tracer's component
// test: drives the shared EntryScreen with the investment config over
// MSW-stubbed investment endpoints and asserts the qty×price contract —
// carry-forward prefill seeds quantity + price with the value computed, Save
// sends only the changed rows (dirty-only) keyed by investment_id with the
// quantity/price_per_unit columns (no client amount), and a 422 marks the row.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { investmentEntryConfig } from "@/components/entry/groups";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      investment_id: "s1",
      display_name: "BBCA",
      currency: "IDR",
      subtype: "stock",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_quantity: "10",
      prefill_price: "1500",
      carried_from: "2026-04",
    },
    {
      investment_id: "g1",
      display_name: "Antam 100g",
      currency: "IDR",
      subtype: "gold",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_quantity: null,
      prefill_price: null,
      carried_from: null,
    },
  ],
};

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={investmentEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-08
it("renders eligible investments grouped by subtype with qty + price prefill and computed value", async () => {
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  // The stock's two tab-stops carry the last snapshot's factors...
  const qty = await screen.findByTestId("investment-entry-quantity-s1");
  expect(qty).toHaveValue("10");
  expect(screen.getByTestId("investment-entry-price-s1")).toHaveValue("1500");
  // ...and the value is computed (10 × 1500 = 15000), not entered.
  expect(screen.getByTestId("investment-entry-value-s1")).toHaveTextContent(/15/);

  // The fresh gold has empty factors and a placeholder computed value.
  expect(screen.getByTestId("investment-entry-quantity-g1")).toHaveValue("");
  expect(screen.getByTestId("investment-entry-price-g1")).toHaveValue("");
  expect(screen.getByTestId("investment-entry-value-g1")).toHaveTextContent("—");
  expect(screen.getByTestId("investment-entry-row-g1")).toHaveTextContent(/no previous value/i);

  // Grouped by subtype: a Stocks section (s1) and a Gold section (g1).
  const stocks = screen.getByTestId("investment-entry-group-stock");
  expect(stocks).toHaveTextContent(/stocks/i);
  expect(within(stocks).getByTestId("investment-entry-row-s1")).toBeInTheDocument();
  const gold = screen.getByTestId("investment-entry-group-gold");
  expect(gold).toHaveTextContent(/gold/i);
  expect(within(gold).getByTestId("investment-entry-row-g1")).toBeInTheDocument();

  // Desktop labels the two tab-stops once per group so a carried-forward row
  // (placeholders gone) still tells units from price. getByText matches only the
  // header span — placeholders/aria-labels on the inputs are not text content.
  expect(within(stocks).getByText("Quantity")).toBeInTheDocument();
  expect(within(stocks).getByText("Price per unit")).toBeInTheDocument();
  // The derived-total column is named too, so it isn't the one unlabelled column.
  expect(within(stocks).getByText("Value")).toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-06
// covers: INV-SNAPSHOTS-08
it("Save sends only changed rows keyed by investment_id with quantity + price_per_unit", async () => {
  let posted: {
    rows: Array<{
      investment_id: string;
      quantity: string;
      price_per_unit: string;
      currency: string;
    }>;
  } | null = null;
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen();
  const user = userEvent.setup();

  // Fill in the fresh gold's factors; leave the stock at its prefill (untouched).
  const goldQty = await screen.findByTestId("investment-entry-quantity-g1");
  await user.type(goldQty, "5");
  await user.type(screen.getByTestId("investment-entry-price-g1"), "2000000");
  // Computed value updates live (5 × 2,000,000).
  await waitFor(() =>
    expect(screen.getByTestId("investment-entry-value-g1")).not.toHaveTextContent("—"),
  );
  await user.click(screen.getByTestId("investment-entry-save"));

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({
    investment_id: "g1",
    quantity: "5",
    price_per_unit: "2000000",
    currency: "IDR",
  });
  // The stored amount is derived server-side, never sent by the client.
  expect(posted!.rows[0]).not.toHaveProperty("amount");
});

// covers: INV-SNAPSHOTS-06
it("a half-filled qty×price row is not dirty and can't be saved", async () => {
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();
  const user = userEvent.setup();

  // Type only the quantity for the fresh gold — no price. Incomplete pair.
  const goldQty = await screen.findByTestId("investment-entry-quantity-g1");
  await user.type(goldQty, "5");

  expect(screen.getByTestId("investment-entry-dirty-count")).toHaveTextContent(/0/);
  expect(screen.getByTestId("investment-entry-save")).toBeDisabled();
});

// covers: INV-SNAPSHOTS-06
it("marks the offending row on a 422 per-row error", async () => {
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/bulk", () =>
      HttpResponse.json({ errors: [{ investment_id: "s1", code: "ineligible" }] }, { status: 422 }),
    ),
  );
  renderScreen();
  const user = userEvent.setup();

  const price = await screen.findByTestId("investment-entry-price-s1");
  await user.clear(price);
  await user.type(price, "1600");
  await user.click(screen.getByTestId("investment-entry-save"));

  expect(await screen.findByTestId("investment-entry-error-s1")).toBeInTheDocument();
});
