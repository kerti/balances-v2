// Per-type conformance for the Receivable detail descriptor (ADR-0051, A2 —
// cross-group, flat entity). No info-card fields — the counterparty/due-date ride
// the header line and the only body content is the shared-surface description.
// Asserts the header line, description, chart, snapshot table, and actions row.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { receivableDescriptor } from "./receivable";
import type { Receivable, ReceivableSnapshot, HouseholdMember } from "@/api/types";
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

const receivable: Receivable = {
  id: "a1",
  household_id: "h1",
  display_name: "Loan to Alex",
  description: "Personal loan, interest-free",
  ownership_type: "sole",
  entry_type: "acquired" as const,
  sole_owner_user_id: "u1",
  native_currency: "USD",
  status: "active",
  terminated_at: null,
  termination_note: null,
  counterparty_name: "Alex Doe",
  due_date: "2027-01-01T00:00:00Z",
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
  tag_id: null,
};

const snapshots: ReceivableSnapshot[] = [
  {
    id: "s2",
    receivable_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "5000.00",
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
    receivable_id: "a1",
    year_month: "2026-05-01T00:00:00Z",
    amount: "6000.00",
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
    http.get("/api/receivables/a1", () => HttpResponse.json(receivable)),
    http.get("/api/receivables/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("receivableDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={receivableDescriptor} assetId="a1" onBack={vi.fn()} />,
    );

    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Loan to Alex");
    // Header line: counterparty · due {date}.
    expect(screen.getByText(/Alex Doe/)).toBeInTheDocument();

    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Personal loan, interest-free")).toBeInTheDocument();

    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/5,?000/)).toBeInTheDocument();

    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("receivable-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/a1/"),
    );
  });
});
