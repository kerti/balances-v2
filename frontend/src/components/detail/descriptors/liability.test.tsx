// Per-type conformance for the Liability detail descriptor (ADR-0051, A2 —
// cross-group). The entity is flat (Position fields sit directly on the row), so
// this also exercises the identity `getAsset`. Asserts the header line, the info
// card (principal/interest/term/period folded into InfoGrid) + description, the
// chart, the snapshot table, and the actions row.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { liabilityDescriptor } from "./liability";
import type { Liability, LiabilitySnapshot, HouseholdMember } from "@/api/types";
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

const liability: Liability = {
  id: "a1",
  household_id: "h1",
  display_name: "Home Mortgage",
  description: "30-year fixed",
  subtype: "institutional",
  ownership_type: "sole",
  sole_owner_user_id: "u1",
  native_currency: "USD",
  status: "active",
  terminated_at: null,
  termination_note: null,
  counterparty_name: "Test Bank",
  principal: "300000.00",
  interest_rate: "4.50",
  term_months: 360,
  start_date: "2020-01-01T00:00:00Z",
  maturity_date: "2050-01-01T00:00:00Z",
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
  tag_id: null,
};

const snapshots: LiabilitySnapshot[] = [
  {
    id: "s2",
    liability_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "280000.00",
    currency: "USD",
    as_of_date: null,
    description: null,
    created_by: "u1",
    created_at: "2026-06-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-06-01T00:00:00Z",
  },
  {
    id: "s1",
    liability_id: "a1",
    year_month: "2026-05-01T00:00:00Z",
    amount: "285000.00",
    currency: "USD",
    as_of_date: null,
    description: null,
    created_by: "u1",
    created_at: "2026-05-01T00:00:00Z",
    updated_by: "u1",
    updated_at: "2026-05-01T00:00:00Z",
  },
];

function stubEndpoints() {
  server.use(
    http.get("/api/liabilities/a1", () => HttpResponse.json(liability)),
    http.get("/api/liabilities/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("liabilityDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={liabilityDescriptor} assetId="a1" onBack={vi.fn()} />,
    );

    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Home Mortgage");
    // Header line: subtype · counterparty.
    expect(screen.getByText(/Test Bank/)).toBeInTheDocument();

    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Principal:")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Interest rate:")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Term:")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Period:")).toBeInTheDocument();
    expect(within(detailsCard).getByText("30-year fixed")).toBeInTheDocument();

    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/280,?000/)).toBeInTheDocument();

    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("liability-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/a1/"),
    );
  });
});
