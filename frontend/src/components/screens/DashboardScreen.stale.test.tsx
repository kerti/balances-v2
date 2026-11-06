// The dashboard's carried-forward warning links to the bulk-entry screen of the
// first stale group, carrying the selected report's month (#792).
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { DashboardScreen } from "@/components/screens/DashboardScreen";
import type { StalePosition } from "@/api/types";

function stalePos(id: string, group: StalePosition["group"], subtype: string): StalePosition {
  return {
    position_id: id,
    name: `Pos ${id}`,
    group,
    subtype,
    last_month: "2026-01-01T00:00:00Z",
  };
}

function report(stale: StalePosition[]) {
  return {
    year_month: "2026-03-01T00:00:00Z",
    generated_at: null,
    reporting_currency: "IDR",
    nw_total: "100",
    nw_assets: "100",
    nw_liabilities: "0",
    nw_receivables: "0",
    nw_investments: "0",
    earned_income_total: null,
    investment_return_total: null,
    asset_value_change: null,
    write_offs: null,
    tracking_changes: null,
    derived_living_expenses: null,
    user_breakdowns: {},
    stale_positions: stale,
    write_off_positions: [],
    tracking_change_positions: [],
    unsettled_terminations: [],
    fx_rates_used: {},
    missing_fx: [],
  };
}

function renderWith(stale: StalePosition[]) {
  server.use(
    http.get("/api/reports", () => HttpResponse.json([report(stale)])),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.get("/api/fx-rates", () => HttpResponse.json([])),
    http.get("/api/me", () => HttpResponse.json({})),
  );
  return renderWithProviders(
    <MemoryRouter>
      <DashboardScreen />
    </MemoryRouter>,
  );
}

it("links to the assets entry screen first, with the report month, whatever the list order", async () => {
  renderWith([stalePos("s1", "investment", "stock"), stalePos("a1", "asset", "bank_account")]);
  const link = await screen.findByTestId("dashboard-stale-enter");
  expect(link).toHaveAttribute("href", "/assets/enter?month=2026-03");
  expect(link).toHaveTextContent("Update now");
});

it("links a bond-only warning to the accrued entry screen", async () => {
  renderWith([stalePos("b1", "investment", "bond")]);
  const link = await screen.findByTestId("dashboard-stale-enter");
  expect(link).toHaveAttribute("href", "/investments/enter/accrued?month=2026-03");
});

it("prefers priced investments over accrued ones", async () => {
  renderWith([stalePos("b1", "investment", "bond"), stalePos("g1", "investment", "gold")]);
  const link = await screen.findByTestId("dashboard-stale-enter");
  expect(link).toHaveAttribute("href", "/investments/enter/prices?month=2026-03");
});

it("shows no warning and no link when nothing is stale", async () => {
  renderWith([]);
  await screen.findByTestId("month-picker-next");
  expect(screen.queryByTestId("dashboard-stale")).not.toBeInTheDocument();
  expect(screen.queryByTestId("dashboard-stale-enter")).not.toBeInTheDocument();
});

it("shows no link when no stale position maps to an entry screen", async () => {
  renderWith([stalePos("x1", "investment", "crypto")]);
  await screen.findByTestId("dashboard-stale");
  expect(screen.queryByTestId("dashboard-stale-enter")).not.toBeInTheDocument();
});
