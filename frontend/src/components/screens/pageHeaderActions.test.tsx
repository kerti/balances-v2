// Create buttons live in the page header (#949, ADR-0050 amendment): Tags,
// Exchange rates and Inflation rates used to park theirs inside the card. The
// Income header is the one exception — its actions sit in a row under the
// title so the subtitle keeps the full width.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { SettingsFxRatesScreen } from "@/components/screens/SettingsFxRatesScreen";
import { SettingsInflationRatesScreen } from "@/components/screens/SettingsInflationRatesScreen";
import { TagsScreen } from "@/components/screens/TagsScreen";
import { IncomeScreen } from "@/components/screens/IncomeScreen";
import type { Me } from "@/hooks/useSession";

const me: Me = {
  id: "u1",
  household_id: "hh-1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: true,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

const originalWidth = window.innerWidth;
afterEach(() => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: originalWidth });
});

function header() {
  return document.querySelector<HTMLElement>('[data-slot="page-header"]')!;
}

function renderScreen(ui: React.ReactElement) {
  return renderWithProviders(<MemoryRouter>{ui}</MemoryRouter>);
}

it("Exchange rates: the New exchange rate button is in the page header, not the card", async () => {
  server.use(
    http.get("/api/me", () => HttpResponse.json(me)),
    http.get("/api/fx-rates", () => HttpResponse.json([])),
    http.get("/api/fx-rates/currencies", () => HttpResponse.json(["USD"])),
  );
  renderScreen(<SettingsFxRatesScreen />);
  const card = await screen.findByTestId("fx-rates-card");
  const button = await screen.findByTestId("fx-new-rate");
  expect(header()).toContainElement(button);
  expect(card).not.toContainElement(button);
});

it("Exchange rates: no create button while multi-currency is off", async () => {
  server.use(
    http.get("/api/me", () => HttpResponse.json({ ...me, multi_currency_enabled: false })),
  );
  renderScreen(<SettingsFxRatesScreen />);
  await screen.findByRole("heading", { level: 1 });
  expect(screen.queryByTestId("fx-new-rate")).not.toBeInTheDocument();
});

it("Inflation rates: the New inflation rate button is in the page header, not the card", async () => {
  server.use(http.get("/api/inflation-rates", () => HttpResponse.json([])));
  renderScreen(<SettingsInflationRatesScreen />);
  const card = await screen.findByTestId("inflation-rates-card");
  const button = screen.getByTestId("inflation-new-rate");
  expect(header()).toContainElement(button);
  expect(card).not.toContainElement(button);
});

it("Tags: the New tag button is in the page header, not the management card", async () => {
  server.use(
    http.get("/api/tags", () => HttpResponse.json([])),
    http.get("/api/tags/breakdown", () =>
      HttpResponse.json({
        year_month: null,
        reporting_currency: "IDR",
        groups: [],
        missing_fx: [],
        fx_rates_used: [],
      }),
    ),
    http.get("/api/fx-rates", () => HttpResponse.json([])),
  );
  renderScreen(<TagsScreen />);
  const button = await screen.findByTestId("new-tag");
  expect(header()).toContainElement(button);
  expect(
    within(screen.getByTestId("tag-list").parentElement!).queryByTestId("new-tag"),
  ).not.toBeInTheDocument();
});

it("Income: the actions sit in a row under the header, not beside the title", async () => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: 1280 });
  server.use(
    http.get("/api/me", () => HttpResponse.json(me)),
    http.get("/api/income", () => HttpResponse.json([])),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen(<IncomeScreen />);
  const actions = await screen.findByTestId("income-actions");
  expect(header()).not.toContainElement(actions);
  expect(header().compareDocumentPosition(actions) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(within(actions).getByRole("button", { name: /new income/i })).toBeInTheDocument();
});
