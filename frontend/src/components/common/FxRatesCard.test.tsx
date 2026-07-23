// Renderer conformance for the Settings ▸ Exchange Rates mobile–web split
// (ADR-0050 B3b, #511). Flips window.innerWidth across the 768px boundary so
// useIsMobile() picks the stacked-card renderer vs the wide table, then asserts
// the divergence contract: only one tree mounts, the rate (the primary figure)
// reads under the shared `fx-rate-value` testid in each, and the mobile card's
// delete control clears the ≥44px tap floor.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { FxRatesCard } from "@/components/common/FxRatesCard";
import type { FxRate } from "@/api/types";
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
  carryover_date_mode: "month_end",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: true,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

const rates: FxRate[] = [
  {
    id: "fx-1",
    household_id: "hh-1",
    year_month: "2026-05",
    currency: "USD",
    rate: "16250",
    created_by: null,
    created_at: "2026-05-31T00:00:00Z",
    updated_by: null,
    updated_at: "2026-05-31T00:00:00Z",
  },
];

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderCard() {
  server.use(
    http.get("/api/fx-rates", () => HttpResponse.json(rates)),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
  return renderWithProviders(<FxRatesCard />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes the rate, meets the 44px tap floor", async () => {
  setViewport(500);
  renderCard();

  // Card renderer mounted, table renderer did not — one tree in the DOM.
  expect(await screen.findByTestId("fx-rate-cards")).toBeInTheDocument();
  expect(screen.queryByTestId("fx-rate-table")).not.toBeInTheDocument();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();

  // Rate (primary figure) reads — as an equation binding it to the reporting
  // currency, so "16250" can't misread as "16250 USD".
  expect(await screen.findByText("1 USD = 16250 IDR")).toBeInTheDocument();
  expect(screen.getByTestId("fx-rate-value")).toHaveTextContent("16250");

  // A11y floor: the delete control is a ≥44px (size-11) tap target.
  expect(screen.getByRole("button", { name: "Delete" })).toHaveClass("size-11");
});

it("desktop: mounts the table renderer under the same primary-figure testid", async () => {
  setViewport(1280);
  renderCard();

  expect(await screen.findByTestId("fx-rate-table")).toBeInTheDocument();
  expect(screen.getByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("fx-rate-cards")).not.toBeInTheDocument();
  expect(screen.getByTestId("fx-rate-value")).toHaveTextContent("16250");
  expect(await screen.findByText("USD → IDR")).toBeInTheDocument();
});

it("add form: shows a live direction hint once a currency code is entered", async () => {
  setViewport(1280);
  renderCard();

  // Wait for the session so the reporting currency (the counterpart) is known.
  await screen.findByTestId("fx-rate-table");

  // No code yet → no hint.
  expect(screen.queryByTestId("fx-rate-hint")).not.toBeInTheDocument();

  // Typing a 3-letter code spells out the direction + counterpart, with "?" for
  // the not-yet-entered rate.
  fireEvent.change(screen.getByLabelText("Currency"), { target: { value: "SGD" } });
  expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = ? IDR");

  // Typing the rate fills it into the equation.
  fireEvent.change(screen.getByLabelText("Rate"), { target: { value: "12000" } });
  expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = 12000 IDR");
});
