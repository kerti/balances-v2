// Per-type conformance for the Bond detail descriptor (ADR-0051, A5 — accrued
// investments). Drives the generic `PositionDetailScreen` with the *real*
// descriptor over MSW-stubbed endpoints and asserts every region the hand-written
// page rendered still renders — the web-parity net. Bond's own shape rides along:
// the accrued snapshot renderer (S3) flows through `HistorySection.renderRow`, and
// the coupon/fee running totals share the transaction toolbar.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bondDescriptor } from "./bond";
import type {
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
  BondDetails,
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
  display_name: "Govt Bond 2030",
  description: "Ten-year government note",
  subtype: "bond",
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
  risk_profile: "low",
  rolled_from_investment_id: null,
  tag_id: null,
};

const details: BondDetails = {
  investment_id: "i1",
  bond_type: "govt_primary",
  issuer: "Treasury Dept",
  coupon_rate: "6.25",
  coupon_frequency: "semi_annual",
  maturity_date: "2030-01-01T00:00:00Z",
  series_code: "SR-015",
  coupon_disposition: "pays_out",
};

const snapshots: InvestmentSnapshot[] = [
  {
    id: "s2",
    investment_id: "i1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "10500.00",
    currency: "USD",
    quantity: null,
    price_per_unit: null,
    accrued_interest: "500.00",
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
    amount: "10000.00",
    currency: "USD",
    quantity: null,
    price_per_unit: null,
    accrued_interest: "0.00",
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
    transaction_type: "coupon",
    transaction_date: "2026-06-15T00:00:00Z",
    currency: "USD",
    description: "Coupon payment",
    amount: "312.50",
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
    amount: "50000.00",
    quantity: "50",
    price_per_unit: "1000.00",
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
    http.get("/api/investments/bonds/i1", () =>
      HttpResponse.json({ investment, details, outstanding_face: "50000.00" }),
    ),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json(transactions)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("bondDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bondDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Title (H1). Series/type/issuer no longer ride a subtitle — they moved into
    // the details-card identity cluster (asserted below).
    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Govt Bond 2030");

    // The investment headline slot (renderHeadline) mounts the shared component.
    expect(screen.getByTestId("investment-headline")).toBeInTheDocument();

    // Details card: identity cluster (series/issuer/type) + face-value field on
    // the left; coupon + disposition moved into the middle headline column; plus
    // ownership/status + the shared-surface description. Face value is scoped to
    // its own field row — the figure also appears as the headline's Total cost.
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/SR-015/)).toBeInTheDocument();
    expect(within(detailsCard).getByText(/Treasury Dept/)).toBeInTheDocument();
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    const faceValueField = within(detailsCard).getByText("Face value").closest("div")!;
    expect(within(faceValueField).getByText(/50,?000/)).toBeInTheDocument();
    // Coupon + disposition now live in the middle headline column.
    const headlineEl = within(detailsCard).getByTestId("investment-headline");
    expect(within(headlineEl).getByText("Coupon")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Ten-year government note")).toBeInTheDocument();

    // Chart card mounts with ≥2 snapshots.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    // Snapshot section: the accrued renderer shows the latest total value.
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/10,?500/)).toBeInTheDocument();

    // Transaction section: renders the ledger rows, the coupon-total glance, and
    // its own search toolbar.
    const txnCard = screen.getByTestId("tour-transactions");
    expect(within(txnCard).getByText("Opening purchase")).toBeInTheDocument();
    expect(within(txnCard).getByText("Coupon payment")).toBeInTheDocument();
    // The coupon value appears twice: the ledger row + the running coupon-total
    // in the shared toolbar (312.50 each, a single coupon).
    expect(within(txnCard).getAllByText(/312/).length).toBeGreaterThanOrEqual(2);
    const search = within(txnCard).getByTestId("txn-search");

    // Search filters the ledger through descriptor wiring (primitive stays neutral).
    await userEvent.type(search, "Coupon");
    expect(within(txnCard).getByText("Coupon payment")).toBeInTheDocument();
    expect(within(txnCard).queryByText("Opening purchase")).not.toBeInTheDocument();

    // Actions row + the export anchor (bond-prefixed).
    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("bond-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/i1/"),
    );
  });
});
