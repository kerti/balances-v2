// Per-type conformance for the TimeDeposit detail descriptor (ADR-0051, A5 — the
// accrued outlier). Drives the generic `PositionDetailScreen` with the *real*
// descriptor over MSW-stubbed endpoints. Two nets: the shared regions at web
// parity (headline whose cost is the flat principal, accrued snapshots, the
// transaction section), and the maturity/rollover tail that must live entirely in
// the `renderBeforeDetails` / `renderAfterDetails` slots — the callout and the
// rollover-chain card with its navigation — never in the core.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, useLocation } from "react-router";
import type { ReactElement } from "react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { timeDepositDescriptor } from "./timeDeposit";
import type {
  Investment,
  InvestmentSnapshot,
  InvestmentTransaction,
  TimeDepositDetails,
  RolloverRef,
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

const baseInvestment: Investment = {
  id: "i1",
  household_id: "h1",
  display_name: "6M Deposit",
  description: "Six-month term deposit",
  subtype: "time_deposit",
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

const details: TimeDepositDetails = {
  investment_id: "i1",
  bank_name: "First Bank",
  principal: "20000.00",
  interest_rate: "4.50",
  term_months: 6,
  placement_date: "2026-01-01T00:00:00Z",
  maturity_date: "2026-07-01T00:00:00Z",
  rollover_policy: "auto_renew_with_interest",
};

const snapshots: InvestmentSnapshot[] = [
  {
    id: "s2",
    investment_id: "i1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "20100.00",
    currency: "USD",
    quantity: null,
    price_per_unit: null,
    accrued_interest: "100.00",
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
    amount: "20000.00",
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

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

// The descriptor's rollover-chain navigation calls `useNavigate`, so every mount
// needs a router — production has one, the test supplies a MemoryRouter + a probe.
function renderTD(ui: ReactElement) {
  return renderWithProviders(
    <MemoryRouter initialEntries={["/investments/time-deposits/i1"]}>
      {ui}
      <LocationProbe />
    </MemoryRouter>,
  );
}

describe("timeDepositDescriptor detail (conformance)", () => {
  it("renders every shared region through the generic shell (no rollover)", async () => {
    server.use(
      http.get("/api/investments/time-deposits/i1", () =>
        HttpResponse.json({
          investment: baseInvestment,
          details,
          rolled_from: null,
          rolled_to: null,
        }),
      ),
      http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
      http.get("/api/investments/i1/transactions", () => HttpResponse.json([])),
      http.get("/api/household/members", () => HttpResponse.json([owner])),
      http.get("/api/me", () => HttpResponse.json(me)),
    );

    renderTD(
      <PositionDetailScreen descriptor={timeDepositDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("6M Deposit");

    // Headline slot mounts (cost = flat principal, wired in the descriptor).
    expect(screen.getByTestId("investment-headline")).toBeInTheDocument();

    // Details card: ownership/status + description; the left column carries the
    // bank identity cluster + interest rate + term; the middle headline column
    // carries risk + the money summary + the Period range and at-maturity policy
    // (in the wider column so the date range fits one line), where principal now
    // reads as Total cost (a time deposit has no separate principal field).
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("First Bank")).toBeInTheDocument();
    const rateField = within(detailsCard).getByText("Interest rate").closest("div")!;
    expect(within(rateField).getByText(/4\.50/)).toBeInTheDocument();
    const termField = within(detailsCard).getByText("Term").closest("div")!;
    expect(within(termField).getByText(/6 months/)).toBeInTheDocument();
    // Placement + maturity now read as one Period range row; at-maturity + all the
    // spec fields are left-column, not the headline.
    expect(within(detailsCard).getByText("Period")).toBeInTheDocument();
    expect(within(detailsCard).getByText("At maturity")).toBeInTheDocument();
    const headlineEl = within(detailsCard).getByTestId("investment-headline");
    expect(within(headlineEl).getByText(/20,?000/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Six-month term deposit")).toBeInTheDocument();

    // Chart + accrued snapshot section render.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/20,?100/)).toBeInTheDocument();

    // Transaction section renders (empty here) through the same primitive.
    expect(screen.getByTestId("tour-transactions")).toBeInTheDocument();

    // No rollover surfaces on a plain deposit — both slots return null.
    expect(screen.queryByTestId("rollover-callout")).not.toBeInTheDocument();
    expect(screen.queryByTestId("rollover-card")).not.toBeInTheDocument();

    expect(screen.getByTestId("time-deposit-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/i1/"),
    );
  });

  it("surfaces the maturity/rollover tail in the before/after-details slots", async () => {
    const maturity: InvestmentTransaction = {
      id: "t1",
      investment_id: "i1",
      transaction_type: "maturity",
      transaction_date: "2026-07-01T00:00:00Z",
      currency: "USD",
      description: "Matured — rolled over",
      amount: "21200.00",
      quantity: null,
      price_per_unit: null,
      principal_amount: "20000.00",
      interest_amount: "1200.00",
      principal_disposition: "rolled_to_new",
      interest_disposition: "rolled_to_new",
      created_by: "u1",
      created_at: "2026-07-01T00:00:00Z",
      updated_by: "u1",
      updated_at: "2026-07-01T00:00:00Z",
    };
    const rolledFrom: RolloverRef = { id: "i0", display_name: "Prior Deposit" };
    const maturedInvestment: Investment = { ...baseInvestment, status: "matured" };

    server.use(
      http.get("/api/investments/time-deposits/i1", () =>
        HttpResponse.json({
          investment: maturedInvestment,
          details,
          rolled_from: rolledFrom,
          rolled_to: null,
        }),
      ),
      http.get("/api/investments/i1/snapshots", () => HttpResponse.json(snapshots)),
      http.get("/api/investments/i1/transactions", () => HttpResponse.json([maturity])),
      http.get("/api/household/members", () => HttpResponse.json([owner])),
      http.get("/api/me", () => HttpResponse.json(me)),
    );

    renderTD(
      <PositionDetailScreen descriptor={timeDepositDescriptor} assetId="i1" onBack={vi.fn()} />,
    );

    // renderBeforeDetails slot: the post-maturity callout with its rolled amount +
    // the link-successor dialog trigger.
    const callout = await screen.findByTestId("rollover-callout");
    expect(within(callout).getByText(/21,?200/)).toBeInTheDocument();

    // renderAfterDetails slot: the rollover-chain card with the "from" link.
    const chainCard = screen.getByTestId("rollover-card");
    const fromLink = within(chainCard).getByTestId("rollover-from-link");
    expect(fromLink).toHaveTextContent("Prior Deposit");

    // Transaction section still renders the maturity ledger row.
    const txnCard = screen.getByTestId("tour-transactions");
    expect(within(txnCard).getByText("Matured — rolled over")).toBeInTheDocument();

    // Navigation is descriptor wiring (useNavigate); the core never learns the route.
    await userEvent.click(fromLink);
    expect(screen.getByTestId("location")).toHaveTextContent("/investments/time-deposits/i0");
  });
});
