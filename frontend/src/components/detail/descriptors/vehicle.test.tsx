// Per-type conformance for the Vehicle detail descriptor (ADR-0051, A2). Same
// amount-only net as Property: header line, info card (depreciation folded into
// InfoGrid) + description, chart, snapshot table, actions row.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { vehicleDescriptor } from "./vehicle";
import type { Asset, AssetSnapshot, VehicleDetails, HouseholdMember } from "@/api/types";
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
  display_name: "Family Car",
  description: "Daily driver",
  subtype: "vehicle",
  ownership_type: "sole",
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

const details: VehicleDetails = {
  asset_id: "a1",
  vehicle_type: "car",
  make: "Toyota",
  model: "Corolla",
  year: 2019,
  plate_number: "ABC 123",
  annual_depreciation_rate: "10.00",
};

const snapshots: AssetSnapshot[] = [
  {
    id: "s2",
    asset_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "18000.00",
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
    amount: "20000.00",
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
    http.get("/api/vehicles/a1", () => HttpResponse.json({ asset, details })),
    http.get("/api/assets/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("vehicleDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={vehicleDescriptor} assetId="a1" onBack={vi.fn()} />,
    );

    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Family Car");
    // Header line joins type · make model · year · plate.
    expect(screen.getByText(/Toyota Corolla/)).toBeInTheDocument();

    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Depreciation rate:")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Daily driver")).toBeInTheDocument();

    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/18,?000/)).toBeInTheDocument();

    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    expect(screen.getByTestId("vehicle-export")).toHaveAttribute(
      "href",
      expect.stringContaining("/a1/"),
    );
  });
});
