// Per-type conformance for the Stock detail descriptor (ADR-0051, A3 — the
// investment mechanism). Stock is the richest investment type: it exercises the
// two investment-only pieces the amount-only linchpin (#525) didn't — the
// `renderHeadline` slot (shared `InvestmentHeadline` fed cost-basis wiring) and a
// multi-section `HistorySection` (qty×price snapshots + a transaction ledger with
// its own search toolbar + reconcile banner). This test drives the generic
// `PositionDetailScreen` with the *real* descriptor over MSW-stubbed endpoints and
// asserts every region the hand-written page rendered still renders — the
// web-parity net for the consolidation.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { stockDescriptor } from "./stock";
import type {
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
  StockDetails,
  HouseholdMember,
} from "@/api/types";
import type { Me } from "@/hooks/useSession";

const owner: HouseholdMember = {
  id: "u1",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
};

const me: Me = {
  id: "u1",
  household_id: "h1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  carryover_date_mode: "month_end",
  time_zone: "UTC",
  reporting_currency: "USD",
  multi_currency_enabled: false,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

const investment: Investment = {
  id: "i1",
  household_id: "h1",
  display_name: "Acme Corp Shares",
  description: "Core equity holding",
  subtype: "stock",
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  status: "active",
  terminated_at: null,
  termination_note: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
  risk_profile: "medium",
  rolled_from_investment_id: null,
  tag_id: null,
};

const details: StockDetails = {
  investment_id: "i1",
  ticker: "ACME",
  exchange: "NYSE",
};

const snapshots: InvestmentSnapshot[] = [
  {
    id: "s2",
    investment_id: "i1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "6600.00",
    currency: "USD",
    quantity: "60",
    price_per_unit: "110.00",
    accrued_interest: null,
    as_of_date: null,
    description: null,
    created_by: "u1",
    created_at: "2026-06-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-06-01T00:00:00Z",
  },
  {
    id: "s1",
    investment_id: "i1",
    year_month: "2026-05-01T00:00:00Z",
    amount: "5000.00",
    currency: "USD",
    quantity: "50",
    price_per_unit: "100.00",
    accrued_interest: null,
    as_of_date: null,
    description: null,
    created_by: "u1",
    created_at: "2026-05-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-05-01T00:00:00Z",
  },
];

const transactions: InvestmentTransaction[] = [
  {
    id: "t2",
    investment_id: "i1",
    transaction_type: "dividend",
    transaction_date: "2026-06-15T00:00:00Z",
    currency: "USD",
    description: "Quarterly dividend",
    amount: "150.00",
    quantity: null,
    price_per_unit: null,
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    created_by: "u1",
    created_at: "2026-06-15T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-06-15T00:00:00Z",
  },
  {
    id: "t1",
    investment_id: "i1",
    transaction_type: "buy",
    transaction_date: "2026-05-01T00:00:00Z",
    currency: "USD",
    description: "Opening purchase",
    amount: "5000.00",
    quantity: "50",
    price_per_unit: "100.00",
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    created_by: "u1",
    created_at: "2026-05-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-05-01T00:00:00Z",
  },
];

function stubEndpoints() {
  server.use(
    http.get("/api/investments/stocks/i1", () => HttpResponse.json({ investment, details })),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json(transactions)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("stockDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Title (H1). Ticker/exchange no longer ride a subtitle here — they moved
    // into the details-card identity cluster (asserted below).
    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Acme Corp Shares");

    // Details card: identity cluster (ticker primary, exchange muted) + the
    // cost/P/L/value headline column (relocated from under the H1) + the
    // ownership/status line + the shared-surface description.
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText("ACME")).toBeInTheDocument();
    expect(within(detailsCard).getByText("NYSE")).toBeInTheDocument();

    // Risk profile rides the card header as a compact shield badge (medium),
    // left of the currency — not a row in the headline column.
    expect(within(detailsCard).getByTestId("risk-profile-medium")).toBeInTheDocument();

    // The headline slot now lives in the details card and carries all three
    // stats: total cost (5,000), P/L (+1,600), and total value (6,600).
    const headlineEl = within(detailsCard).getByTestId("investment-headline");
    expect(within(headlineEl).getByText(/5,?000/)).toBeInTheDocument();
    expect(within(headlineEl).getByTestId("investment-headline-pl")).toHaveTextContent(/1,?600/);
    expect(within(headlineEl).getByTestId("investment-headline-value")).toHaveTextContent(/6,?600/);
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Core equity holding")).toBeInTheDocument();

    // Chart card mounts with ≥2 snapshots.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    // Snapshot section: the qty×price renderer shows the latest total value.
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/6,?600/)).toBeInTheDocument();

    // Transaction section: renders the ledger rows + its own search toolbar.
    const txnCard = screen.getByTestId("tour-transactions");
    expect(within(txnCard).getByText("Opening purchase")).toBeInTheDocument();
    expect(within(txnCard).getByText("Quarterly dividend")).toBeInTheDocument();
    const search = within(txnCard).getByTestId("txn-search");
    expect(search).toBeInTheDocument();

    // Search filters the ledger through descriptor wiring (primitive stays neutral).
    await userEvent.type(search, "dividend");
    expect(within(txnCard).getByText("Quarterly dividend")).toBeInTheDocument();
    expect(within(txnCard).queryByText("Opening purchase")).not.toBeInTheDocument();

    // Actions row + the export anchor (stock-prefixed).
    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("stock-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/i1/"),
    );
  });
});
