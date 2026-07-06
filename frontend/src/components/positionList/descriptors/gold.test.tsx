// Per-type conformance for the Gold descriptor on the investment preset
// (ADR-0043). Drives the generic core with the real descriptor over MSW and
// asserts the investment surface wires up: the list loads, the risk badge +
// subtype (form/purity) + activity columns render, and the cost/P-L headline
// shows. Core list behaviour is proven generically in PositionListScreen.test.tsx.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionListScreen } from "@/components/positionList/PositionListScreen";
import { goldDescriptor } from "./gold";
import type { GoldListItem, Investment, InvestmentSnapshot } from "@/api/types";

const investment: Investment = {
  id: "g1",
  household_id: "h1",
  display_name: "Gold Bars",
  description: "Vault holding",
  subtype: "gold",
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  risk_profile: "medium",
  rolled_from_investment_id: null,
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const snapshot: InvestmentSnapshot = {
  id: "s1",
  investment_id: "g1",
  year_month: "2026-06-01T00:00:00Z",
  amount: "4321.00",
  currency: "USD",
  quantity: null,
  price_per_unit: null,
  accrued_interest: null,
  as_of_date: null,
  description: null,
  created_by: "u1",
  created_at: "2026-06-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-06-01T00:00:00Z",
};

const goldItem: GoldListItem = {
  investment,
  details: { investment_id: "g1", form: "bar", purity: "999" },
  latest_snapshot: snapshot,
  cost_basis: "4000.00",
  transaction_count: 3,
  last_transaction_date: "2026-05-01T00:00:00Z",
};

describe("goldDescriptor (conformance)", () => {
  it("loads and surfaces the investment columns + headline", async () => {
    server.use(
      http.get("/api/investments/golds", () => HttpResponse.json([goldItem])),
      http.get("/api/investments/time-series", () => HttpResponse.json([])),
    );
    renderWithProviders(<PositionListScreen descriptor={goldDescriptor} onSelect={vi.fn()} />);

    const row = await screen.findByTestId("gold-row");
    // Shared surface: name, status, value.
    expect(within(row).getByText("Gold Bars")).toBeInTheDocument();
    expect(within(row).getByText("Active")).toBeInTheDocument();
    expect(within(row).getByText(/4,?321/)).toBeInTheDocument();
    // Subtype column: form label + purity.
    expect(within(row).getByText("Bar")).toBeInTheDocument();
    // Description renders as the secondary line.
    expect(within(row).getByText("Vault holding")).toBeInTheDocument();
    // Activity column: ledger transaction count.
    expect(within(row).getByText("3 transactions")).toBeInTheDocument();
    // Declared subtype header + the cost/P-L headline slot.
    expect(screen.getByRole("columnheader", { name: /Form & purity/ })).toBeInTheDocument();
    expect(screen.getByTestId("gold-total")).toBeInTheDocument();
  });
});
