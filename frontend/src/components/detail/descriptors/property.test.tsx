// Per-type conformance for the Property detail descriptor (ADR-0051, A2). Drives
// the generic `PositionDetailScreen` with the real descriptor over MSW-stubbed
// endpoints and asserts every region renders — the header line, the info card
// (acquisition + appreciation folded into the shared InfoGrid) + description, the
// chart, the snapshot table, and the actions row. The web-parity net for the
// amount-only migration.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { propertyDescriptor } from "./property";
import type { Asset, AssetSnapshot, PropertyDetails, HouseholdMember } from "@/api/types";
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

const asset: Asset = {
  id: "a1",
  household_id: "h1",
  display_name: "Family Home",
  description: "Primary residence",
  subtype: "property",
  ownership_type: "sole",
  entry_type: "acquired" as const,
  sole_owner_user_id: "u1",
  native_currency: "USD",
  tag_id: null,
  status: "active",
  terminated_at: null,
  termination_note: null,
  created_by: "u1",
  created_at: "2026-01-01T00:00:00Z",
  updated_by: "u1",
  updated_at: "2026-01-01T00:00:00Z",
};

const details: PropertyDetails = {
  asset_id: "a1",
  property_type: "house",
  address: "12 Oak Street",
  acquisition_date: "2020-01-01T00:00:00Z",
  acquisition_cost: "500000.00",
  annual_appreciation_rate: "3.5",
};

const snapshots: AssetSnapshot[] = [
  {
    id: "s2",
    asset_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "620000.00",
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
    asset_id: "a1",
    year_month: "2026-05-01T00:00:00Z",
    amount: "600000.00",
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
    http.get("/api/properties/a1", () => HttpResponse.json({ asset, details })),
    http.get("/api/assets/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("propertyDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={propertyDescriptor} assetId="a1" onBack={vi.fn()} />,
    );

    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Family Home");
    expect(screen.getByText(/12 Oak Street/)).toBeInTheDocument();

    // Details card: ownership/status line, the info-grid labels (acquisition +
    // appreciation), and the shared-surface description.
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Acquired")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Appreciation rate")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Primary residence")).toBeInTheDocument();

    // Chart card mounts with ≥2 snapshots.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    // Snapshot table: the S1 row renderer shows the amount.
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/620,?000/)).toBeInTheDocument();

    // Actions row: edit/delete + the export anchor.
    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("property-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/a1/"),
    );
  });
});
