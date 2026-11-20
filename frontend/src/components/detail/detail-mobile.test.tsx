// Detail-page mobile conformance (ADR-0051 Phase B, INV-PRESENTATION-08). With
// `useIsMobile` forced true, the generic shell driven by the real BankAccount
// descriptor must mount the snapshot *card* renderer (not the wide table),
// promote the amount to the card headline under the shared `snapshot-amount`
// testid, and size the row ⋮ action to the 44px tap floor. This is the
// renderer-independent net: the same descriptor, the same testids, a different
// renderer at phone width.
//
// covers: INV-PRESENTATION-08
import { describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { PositionDetailScreen } from "@/components/detail/PositionDetailScreen";
import { bankAccountDescriptor } from "@/components/detail/descriptors/bankAccount";
import type { Asset, AssetSnapshot, BankAccountDetails, HouseholdMember } from "@/api/types";
import type { Me } from "@/hooks/useSession";

vi.mock("@/hooks/use-mobile", () => ({ useIsMobile: () => true }));

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

const details: BankAccountDetails = {
  asset_id: "a1",
  bank_name: "Test Bank",
  account_number: "1234567890",
  account_type: "savings",
};

const snapshots: AssetSnapshot[] = [
  {
    id: "s1",
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
];

function stubEndpoints() {
  server.use(
    http.get("/api/bank-accounts/a1", () => HttpResponse.json({ asset, details })),
    http.get("/api/assets/a1/snapshots", () => HttpResponse.json(snapshots)),
    http.get("/api/household/members", () => HttpResponse.json([owner])),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
}

describe("detail mobile a11y floor (amount-only)", () => {
  it("mounts the snapshot card renderer and floors the row action at phone width", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
      { route: "/" },
    );

    // Renderer flips to cards; the wide snapshot table is gone.
    const cards = await screen.findByTestId("tour-snapshots-cards");
    expect(screen.queryByTestId("tour-snapshots-table")).not.toBeInTheDocument();

    // Primary value promoted to the card headline under the shared testid.
    expect(within(cards).getByTestId("snapshot-amount")).toHaveTextContent(/4,?321/);

    // Labelled key/value rows, month before the amount (#849).
    const month = within(cards).getByText("Month");
    const balance = within(cards).getByText("Balance");
    expect(month.compareDocumentPosition(balance) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

    // Tap-target floor: the row ⋮ action carries the 44px size class.
    const action = within(cards).getByRole("button", { name: /snapshot actions/i });
    expect(action).toHaveClass("size-11");
  });

  it("floors the header/action-row secondary controls at the 44px tap target (#542)", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
      { route: "/" },
    );

    // The secondary controls left at `size-sm` after B1 (which floored only the
    // single promoted primary action) — the snapshot-section header Export /
    // Import triggers and the actions-row Help / Record snapshot / Terminate / Delete —
    // now inherit the primitive floor (`max-md:min-h-11` on the Button text
    // sizes, #559): ≥44px on phones, natural size
    // (32px) from 768px up. The row ⋮ (above) rides its own `size-11` floor.
    const secondary = [
      await screen.findByTestId("bank-account-export"),
      screen.getByTestId("import-snapshots-trigger"),
      screen.getByTestId("snapshot-record"),
    ];
    for (const control of secondary) {
      expect(control).toHaveClass("max-md:min-h-11");
    }
    // The header's two icon buttons (Help ?, the ⋯ menu) are square: they floor
    // with `max-md:size-11` instead (#839).
    for (const icon of [
      screen.getByTestId("help-tour"),
      screen.getByTestId("position-menu-trigger"),
    ]) {
      expect(icon).toHaveClass("max-md:size-11");
    }
  });

  it("makes Record snapshot the header primary; the card keeps only the I/O row (#935)", async () => {
    stubEndpoints();
    renderWithProviders(
      <PositionDetailScreen descriptor={bankAccountDescriptor} assetId="a1" onBack={vi.fn()} />,
      { route: "/" },
    );

    // The header's one primary; Edit moved into the ⋯ menu.
    const actions = await screen.findByTestId("tour-actions");
    expect(within(actions).getByRole("button", { name: "Record snapshot" })).toBeInTheDocument();
    expect(within(actions).queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
    // The snapshots card no longer has its own record button.
    expect(screen.queryByTestId("snapshot-create-row")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("snapshot-record")).toHaveLength(1);

    // Row 2 — I/O: Export + Import ride the second row.
    const ioRow = screen.getByTestId("snapshot-io-row");
    expect(within(ioRow).getByTestId("bank-account-export")).toBeInTheDocument();
    expect(within(ioRow).getByTestId("import-snapshots-trigger")).toBeInTheDocument();
  });
});
