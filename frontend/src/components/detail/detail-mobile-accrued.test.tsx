// Detail-page mobile conformance for the accrued snapshot shape (ADR-0051 Phase
// B, INV-PRESENTATION-08 — #537). The two accrued detail pages (Bond,
// TimeDeposit) share one `AccruedInterestSnapshotRow`/`Card` pair off the generic
// shell, so the table→card flip is proven once here on Bond, the representative
// subtype. With `useIsMobile` forced true the snapshot section must mount the
// *card* renderer (not the wide table), promote the total value to the card
// headline under the shared `snapshot-amount` testid, surface the principal ·
// accrued secondary line, and floor the row ⋮ action at 44px. A second pass at
// desktop width proves the same `snapshot-row` / `snapshot-amount` anchors ride
// the table row — the renderer-neutral net the ADR requires.
//
// covers: INV-PRESENTATION-08
import { describe, it, expect, vi, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bondDescriptor } from "@/components/detail/descriptors/bond";
import { useIsMobile } from "@/hooks/use-mobile";
import type { Investment, InvestmentSnapshot, BondDetails, HouseholdMember } from "@/api/types";
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
  display_name: "Govt Bond 2030",
  description: "Ten-year government note",
  subtype: "bond",
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
    id: "s1",
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
];

function stubEndpoints() {
  server.use(
    http.get("/api/investments/bonds/i1", () =>
      HttpResponse.json({ investment, details, outstanding_face: "50000.00" }),
    ),
    http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/investments/i1/transactions", () => HttpResponse.json([])),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("detail mobile a11y floor (accrued)", () => {
  beforeEach(() => mockUseIsMobile.mockReset());

  it("mounts the snapshot card renderer and floors the row action at phone width", async () => {
    mockUseIsMobile.mockReturnValue(true);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bondDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Renderer flips to cards; the wide snapshot table is gone.
    const cards = await screen.findByTestId("tour-snapshots-cards");
    expect(screen.queryByTestId("tour-snapshots-table")).not.toBeInTheDocument();

    // Total value promoted to the card headline under the shared testid.
    expect(within(cards).getByTestId("snapshot-amount")).toHaveTextContent(/10,?500/);

    // The principal · accrued split reads on the card (principal = 10500 − 500).
    expect(within(cards).getByText(/Principal .*10,?000/)).toBeInTheDocument();
    expect(within(cards).getByText(/Accrued .*500/)).toBeInTheDocument();

    // Tap-target floor: the row ⋮ action carries the 44px size class.
    const action = within(cards).getByRole("button", { name: /snapshot actions/i });
    expect(action).toHaveClass("size-11");
  });

  it("rides the same snapshot-row / snapshot-amount anchors on the desktop table", async () => {
    mockUseIsMobile.mockReturnValue(false);
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bondDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // Desktop stays on the wide table — no card list.
    const table = await screen.findByTestId("tour-snapshots-table");
    expect(screen.queryByTestId("tour-snapshots-cards")).not.toBeInTheDocument();

    // Same anchors both renderers expose: the row and its promoted value.
    const row = within(table).getByTestId("snapshot-row");
    expect(within(row).getByTestId("snapshot-amount")).toHaveTextContent(/10,?500/);
    expect(within(row).getByRole("button", { name: /snapshot actions/i })).toBeInTheDocument();
  });
});
