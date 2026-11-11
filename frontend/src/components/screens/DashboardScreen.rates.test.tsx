// The dashboard's rates line (one muted line under the headline, with the
// carry-forward caveat inline) and the "Also in" selector (#851).
import { afterEach, describe, it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import i18n from "@/i18n";
import { DashboardScreen, SECONDARY_CURRENCY_KEY } from "@/components/screens/DashboardScreen";
import type { FxRate } from "@/api/types";

function report(fxUsed: Record<string, string>) {
  return {
    year_month: "2026-03-01T00:00:00Z",
    generated_at: null,
    reporting_currency: "IDR",
    nw_total: "1000000",
    nw_assets: "1000000",
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
    stale_positions: [],
    write_off_positions: [],
    tracking_change_positions: [],
    unsettled_terminations: [],
    fx_rates_used: fxUsed,
    missing_fx: [],
  };
}

function fx(currency: string, yearMonth: string, rate: string): FxRate {
  return {
    id: `${currency}-${yearMonth}`,
    household_id: "h",
    year_month: `${yearMonth}-01T00:00:00Z`,
    currency,
    rate,
    created_by: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_by: null,
    updated_at: "2026-01-01T00:00:00Z",
  } as FxRate;
}

function renderDashboard({
  fxUsed = {},
  rates = [],
  multi = true,
}: {
  fxUsed?: Record<string, string>;
  rates?: FxRate[];
  multi?: boolean;
}) {
  server.use(
    http.get("/api/reports", () => HttpResponse.json([report(fxUsed)])),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.get("/api/fx-rates", () => HttpResponse.json(rates)),
    http.get("/api/me", () => HttpResponse.json({ multi_currency_enabled: multi })),
  );
  return renderWithProviders(
    <MemoryRouter>
      <DashboardScreen />
    </MemoryRouter>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
});

describe("rates line", () => {
  it("shows the month's rate in one line and no rates card", async () => {
    renderDashboard({ fxUsed: { USD: "16450" }, rates: [fx("USD", "2026-03", "16450")] });
    const line = await screen.findByTestId("dashboard-rates");
    expect(line).toHaveTextContent("Rates: 1 USD = 16,450 IDR");
    expect(screen.queryByTestId("dashboard-rate-carried-USD")).not.toBeInTheDocument();
    expect(screen.queryByText("Exchange rates this month")).not.toBeInTheDocument();
  });

  it("is absent when no foreign rate was used", async () => {
    renderDashboard({});
    // First test in the file pays the screen's cold render; under a loaded
    // `make check` that ran past findBy's 1 s default.
    await screen.findByTestId("dashboard-headline", {}, { timeout: 5000 });
    expect(screen.queryByTestId("dashboard-rates")).not.toBeInTheDocument();
  });

  it("names the carry-forward source on the same line", async () => {
    renderDashboard({ fxUsed: { USD: "16450" }, rates: [fx("USD", "2026-01", "16450")] });
    const carried = await screen.findByTestId("dashboard-rate-carried-USD");
    expect(carried).toHaveTextContent("· carried forward from Jan 2026");
    expect(carried).toHaveClass("text-warning");
    expect(screen.getByTestId("dashboard-rates")).toContainElement(carried);
  });

  it("joins several currencies with ' · ', flagging only the carried one", async () => {
    renderDashboard({
      fxUsed: { EUR: "17500", USD: "16450" },
      rates: [fx("USD", "2026-03", "16450"), fx("EUR", "2026-02", "17500")],
    });
    const line = await screen.findByTestId("dashboard-rates");
    expect(line).toHaveTextContent(
      "Rates: 1 EUR = 17,500 IDR · carried forward from Feb 2026 · 1 USD = 16,450 IDR",
    );
    expect(within(line).queryByTestId("dashboard-rate-carried-USD")).not.toBeInTheDocument();
  });

  it("formats numbers in the Indonesian locale", async () => {
    await i18n.changeLanguage("id-ID");
    try {
      renderDashboard({ fxUsed: { USD: "16450" }, rates: [fx("USD", "2026-03", "16450")] });
      const line = await screen.findByTestId("dashboard-rates");
      expect(line).toHaveTextContent("Kurs: 1 USD = 16.450 IDR");
    } finally {
      await i18n.changeLanguage("en-GB");
    }
  });
});

describe("Also in", () => {
  const rates = [fx("USD", "2026-03", "16450")];

  it("is label-first and starts at 'Pick a currency'", async () => {
    renderDashboard({ fxUsed: { USD: "16450" }, rates });
    const select = await screen.findByTestId("dashboard-secondary-currency");
    expect(select).toHaveValue("");
    expect(within(select).getByRole("option", { name: "Pick a currency" })).toBeInTheDocument();
    expect(select.closest("label")).toHaveTextContent(/^Also in/);
    expect(screen.queryByTestId("dashboard-secondary-amount")).not.toBeInTheDocument();
  });

  it("is hidden when multi-currency is off", async () => {
    renderDashboard({ rates, multi: false });
    await screen.findByTestId("dashboard-headline");
    expect(screen.queryByTestId("dashboard-secondary-currency")).not.toBeInTheDocument();
  });

  it("keeps the choice across a remount", async () => {
    const user = userEvent.setup();
    const first = renderDashboard({ fxUsed: { USD: "16450" }, rates });
    await user.selectOptions(await screen.findByTestId("dashboard-secondary-currency"), "USD");
    expect(await screen.findByTestId("dashboard-secondary-amount")).toHaveTextContent("≈");
    expect(localStorage.getItem(SECONDARY_CURRENCY_KEY)).toBe("USD");
    first.unmount();

    renderDashboard({ fxUsed: { USD: "16450" }, rates });
    expect(await screen.findByTestId("dashboard-secondary-currency")).toHaveValue("USD");
    expect(await screen.findByTestId("dashboard-secondary-amount")).toHaveTextContent("≈");
  });

  it("falls back to unpicked when the remembered currency has no rate", async () => {
    localStorage.setItem(SECONDARY_CURRENCY_KEY, "EUR");
    renderDashboard({ fxUsed: { USD: "16450" }, rates });
    expect(await screen.findByTestId("dashboard-secondary-currency")).toHaveValue("");
  });

  it("still renders and works when storage throws", async () => {
    const user = userEvent.setup();
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    renderDashboard({ fxUsed: { USD: "16450" }, rates });
    const select = await screen.findByTestId("dashboard-secondary-currency");
    expect(select).toHaveValue("");
    await user.selectOptions(select, "USD");
    expect(await screen.findByTestId("dashboard-secondary-amount")).toHaveTextContent("≈");
  });
});
