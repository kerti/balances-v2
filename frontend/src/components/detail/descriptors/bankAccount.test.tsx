// Per-type conformance for the BankAccount detail descriptor (ADR-0051, A1). It
// drives the generic `PositionDetailScreen` with the *real* descriptor over
// MSW-stubbed endpoints and asserts the descriptor renders the expected regions:
// the header line, the details card, the chart (≥2 snapshots), the snapshot
// table, and the actions row (edit/delete/terminate/export + tour anchors). The
// test fails if a region is dropped — the web-parity net for the consolidation.
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bankAccountDescriptor } from "./bankAccount";
import type { Asset, AssetSnapshot, BankAccountDetails, HouseholdMember } from "@/api/types";
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
  display_name: "Everyday Checking",
  description: "Primary current account",
  subtype: "bank_account",
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

const details: BankAccountDetails = {
  asset_id: "a1",
  bank_name: "Test Bank",
  account_number: "1234567890",
  account_type: "savings",
};

const snapshots: AssetSnapshot[] = [
  {
    id: "s2",
    asset_id: "a1",
    year_month: "2026-06-01T00:00:00Z",
    amount: "4321.00",
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
    amount: "4000.00",
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
    http.get("/api/bank-accounts/a1", () => HttpResponse.json({ asset, details })),
    http.get("/api/assets/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("bankAccountDescriptor detail (conformance)", () => {
  it("renders every region through the generic shell", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
    );

    // Title resolves once loaded; the bank identity moved from a header subtitle
    // into the details card as its own rows (ADR-0051 Phase B).
    const title = await screen.findByTestId("tour-overview");
    expect(title).toHaveTextContent("Everyday Checking");

    // Details card: title, ownership/status line, and the shared-surface
    // description paragraph.
    const detailsCard = screen.getByTestId("tour-details");
    expect(within(detailsCard).getByText(/Pat Owner \(you\)/)).toBeInTheDocument();
    expect(within(detailsCard).getByText("Active")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Primary current account")).toBeInTheDocument();
    // Bank identity now lives here as its own label/value rows.
    expect(within(detailsCard).getByText("Test Bank")).toBeInTheDocument();
    expect(within(detailsCard).getByText("1234567890")).toBeInTheDocument();
    expect(within(detailsCard).getByText("Savings")).toBeInTheDocument();

    // Chart card mounts with ≥2 snapshots.
    expect(screen.getByTestId("tour-chart")).toBeInTheDocument();

    // Snapshot table: the S1 row renderer shows the amount.
    const snapshotsCard = screen.getByTestId("tour-snapshots");
    expect(within(snapshotsCard).getByText(/4,?321/)).toBeInTheDocument();

    // Actions row: help/edit/terminate/delete + the export anchor.
    const actions = screen.getByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: /edit/i })).toBeInTheDocument();
    expect(within(actions).getByRole("button", { name: /delete/i })).toBeInTheDocument();
    const exportLink = screen.getByTestId("bank-account-export");
    expect(exportLink).toHaveAttribute("href", expect.stringContaining("/a1/"));
  });
});
