// Detail-page mobile conformance for the investment transaction-ledger shape
// (ADR-0051 Phase B, INV-PRESENTATION-08 — #538). All five investment detail
// pages (Stock, MutualFund, Gold, Bond, TimeDeposit) share one
// `TransactionRow`/`Card` pair off the generic shell, so the table→card flip is
// proven once here on Stock, the representative subtype. With `useIsMobile`
// forced true the transaction section must mount the *card* renderer (not the
// wide table), promote the cash impact to the card headline under the shared
// `transaction-amount` testid, surface the type + quantity×price detail line,
// and floor the row ⋮ action at 44px. A second pass at desktop width proves the
// same `transaction-row` / `transaction-amount` anchors ride the table row — the
// renderer-neutral net the ADR requires.
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
import type {
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
  StockDetails,
  HouseholdMember,
} from "@/api/types";
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
  entry_type: "acquired" as const,
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

const snapshots: InvestmentSnapshot[] = [];

// One Buy: 60 sh @ 110 = 6,600 cash out.
const transactions: InvestmentTransaction[] = [
  {
    id: "t1",
    investment_id: "i1",
    transaction_type: "buy",
    transaction_date: "2026-06-15T00:00:00Z",
    currency: "USD",
    description: "Opening lot",
    amount: "6600.00",
    quantity: "60",
    price_per_unit: "110.00",
    principal_amount: null,
    interest_amount: null,
    principal_disposition: null,
    interest_disposition: null,
    created_by: "u1",
    created_at: "2026-06-15T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-06-15T00:00:00Z",
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

describe("detail mobile a11y floor (transaction ledger)", () => {
  beforeEach(() => mockUseIsMobile.mockReset());

  it("mounts the transaction card renderer and floors the row action at phone width", async () => {
    mockUseIsMobile.mockReturnValue(true);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Renderer flips to cards; the wide transaction table is gone.
    const cards = await screen.findByTestId("tour-transactions-cards");
    expect(screen.queryByTestId("tour-transactions-table")).not.toBeInTheDocument();

    // Cash impact promoted to the card headline under the shared testid — a Buy
    // reads as a signed cash-out.
    const amount = within(cards).getByTestId("transaction-amount");
    expect(amount).toHaveTextContent(/6,?600/);
    expect(amount).toHaveTextContent(/−|-/);

    // The type + quantity×price detail line reads on the card.
    expect(within(cards).getByText(/60 sh @ .*110/)).toBeInTheDocument();

    // Tap-target floor: the row ⋮ action carries the 44px size class.
    const action = within(cards).getByRole("button", { name: /transaction actions/i });
    expect(action).toHaveClass("size-11");
  });

  it("rides the same transaction-row / transaction-amount anchors on the desktop table", async () => {
    mockUseIsMobile.mockReturnValue(false);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={stockDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Desktop stays on the wide table — no card list.
    const table = await screen.findByTestId("tour-transactions-table");
    expect(screen.queryByTestId("tour-transactions-cards")).not.toBeInTheDocument();

    // Same anchors both renderers expose: the row and its promoted value.
    const row = within(table).getByTestId("transaction-row");
    expect(within(row).getByTestId("transaction-amount")).toHaveTextContent(/6,?600/);
    expect(within(row).getByRole("button", { name: /transaction actions/i })).toBeInTheDocument();
  });
});
