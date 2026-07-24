// Per-type conformance for the Gold detail descriptor (ADR-0051, A4 — qty×price
// completion). A mechanical repeat of the A3 Stock conformance: it drives the
// generic `PositionDetailScreen` with the *real* descriptor over MSW-stubbed
// endpoints and asserts every region the hand-written page rendered still
// renders — the web-parity net for the consolidation. Gold's own nuances (no
// cash-income event; form/purity header line) ride along.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { goldDescriptor } from "./gold";
import type {
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
  GoldDetails,
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
  display_name: "Bullion Holding",
  description: "Physical gold reserve",
  subtype: "gold",
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

const details: GoldDetails = {
  investment_id: "i1",
  form: "bar",
  purity: "0.999",
};

const snapshots: InvestmentSnapshot[] = [
  {
    id: "s2",
    investment_id: "i1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "6600.00",
    currency: "USD",
    quantity: "100",
    price_per_unit: "66.00",
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
    quantity: "100",
    price_per_unit: "50.00",
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
    transaction_type: "sell",
    transaction_date: "2026-06-15T00:00:00Z",
    currency: "USD",
    description: "Partial sale",
    amount: "1320.00",
    quantity: "20",
    price_per_unit: "66.00",
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
    quantity: "100",
    price_per_unit: "50.00",
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
    http.get("/api/investments/golds/i1", () => HttpResponse.json({ investment, details })),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json(transactions)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("goldDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={goldDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Title + header secondary line (form · purity slot).
    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Bullion Holding");
    expect(screen.getByText(/Bar · 24K/)).toBeInTheDocument();

    // The investment headline slot (renderHeadline) mounts the shared component.
    expect(screen.getByTestId("investment-headline")).toBeInTheDocument();

    // Details card: ownership/status line + the shared-surface description.
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Physical gold reserve")).toBeInTheDocument();

    // Chart card mounts with ≥2 snapshots.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    // Snapshot section: the qty×price renderer shows the latest total value.
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/6,?600/)).toBeInTheDocument();

    // Transaction section: renders the ledger rows + its own search toolbar.
    const txnCard = screen.getByTestId("tour-transactions");
    expect(within(txnCard).getByText("Opening purchase")).toBeInTheDocument();
    expect(within(txnCard).getByText("Partial sale")).toBeInTheDocument();
    const search = within(txnCard).getByTestId("txn-search");
    expect(search).toBeInTheDocument();

    // Search filters the ledger through descriptor wiring (primitive stays neutral).
    await userEvent.type(search, "Partial");
    expect(within(txnCard).getByText("Partial sale")).toBeInTheDocument();
    expect(within(txnCard).queryByText("Opening purchase")).not.toBeInTheDocument();

    // Actions row + the export anchor (gold-prefixed).
    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("gold-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/i1/"),
    );
  });
});
