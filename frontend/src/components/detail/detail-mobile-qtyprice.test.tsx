// Detail-page mobile conformance for the qty×price snapshot shape (ADR-0051
// Phase B, INV-PRESENTATION-08 — #536). The three qty×price detail pages (Stock,
// MutualFund, Gold) share one `QuantityPriceSnapshotRow`/`Card` pair off the
// generic shell, so the table→card flip is proven once here on Stock, the
// representative subtype. With `useIsMobile` forced true the snapshot section
// must mount the *card* renderer (not the wide table), promote the position value
// to the card headline under the shared `snapshot-amount` testid, surface the
// quantity × unit-price secondary line, and floor the row ⋮ action at 44px. A
// second pass at desktop width proves the same `snapshot-row` / `snapshot-amount`
// anchors ride the table row — the renderer-neutral net the ADR requires.
//
// covers: INV-PRESENTATION-08
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { stockDescriptor } from "@/components/detail/descriptors/stock";
import { useIsMobile } from "@/hooks/use-mobile";
import type { Investment, InvestmentSnapshot, StockDetails, HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";

vi.mock("@/hooks/use-mobile", () => ({ useIsMobile: vi.fn() }));
const mockUseIsMobile = vi.mocked(useIsMobile);

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
    id: "s1",
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
];

function stubEndpoints() {
  server.use(
    http.get("/api/investments/stocks/i1", () => HttpResponse.json({ investment, details })),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json([])),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("detail mobile a11y floor (qty×price)", () => {
  beforeEach(() => mockUseIsMobile.mockReset());

  it("mounts the snapshot card renderer and floors the row action at phone width", async () => {
    mockUseIsMobile.mockReturnValue(true);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Renderer flips to cards; the wide snapshot table is gone.
    const cards = await screen.findByTestId("tour-snapshots-cards");
    expect(screen.queryByTestId("tour-snapshots-table")).not.toBeInTheDocument();

    // Position value promoted to the card headline under the shared testid.
    expect(within(cards).getByTestId("snapshot-amount")).toHaveTextContent(/6,?600/);

    // The quantity × unit-price secondary line reads on the card.
    expect(within(cards).getByText(/60 sh × .*110/)).toBeInTheDocument();

    // Tap-target floor: the row ⋮ action carries the 44px size class.
    const action = within(cards).getByRole("button", { name: /snapshot actions/i });
    expect(action).toHaveClass("size-11");

    // #542: with a prior snapshot to copy, Copy carryover is the promoted
    // primary of the create row — the large tap target (`h-11 md:h-8`) — and
    // New drops to the secondary outline floor (`min-h-11 md:min-h-0`). (B1 had
    // left the qty×price triggers at the bare `size-sm` height entirely.)
    const carryover = screen.getByTestId("snapshot-carryover");
    expect(carryover).toHaveClass("h-11");
    expect(carryover).toHaveClass("md:h-8");
    const create = screen.getByRole("button", { name: /^new$/i });
    expect(create).toHaveClass("min-h-11");
    expect(create).toHaveClass("md:min-h-0");

    // #542: the transactions header also splits into two rows — trades (Buy
    // primary, Sell) and cash flows (Dividend, Fee).
    const trades = screen.getByTestId("txn-trades-row");
    const cashflow = screen.getByTestId("txn-cashflow-row");
    const buy = within(trades).getByRole("button", { name: /buy/i });
    expect(buy).toHaveClass("h-11", "md:h-8");
    expect(within(trades).getByRole("button", { name: /sell/i })).toHaveClass("min-h-11");
    expect(within(cashflow).getByRole("button", { name: /dividend/i })).toBeInTheDocument();
    expect(within(cashflow).getByRole("button", { name: /fee/i })).toBeInTheDocument();
  });

  it("rides the same snapshot-row / snapshot-amount anchors on the desktop table", async () => {
    mockUseIsMobile.mockReturnValue(false);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Desktop stays on the wide table — no card list.
    const table = await screen.findByTestId("tour-snapshots-table");
    expect(screen.queryByTestId("tour-snapshots-cards")).not.toBeInTheDocument();

    // Same anchors both renderers expose: the row and its promoted value.
    const row = within(table).getByTestId("snapshot-row");
    expect(within(row).getByTestId("snapshot-amount")).toHaveTextContent(/6,?600/);
    expect(within(row).getByRole("button", { name: /snapshot actions/i })).toBeInTheDocument();
  });
});
