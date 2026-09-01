// Renderer conformance for the Settings ▸ Inflation Rates mobile–web split
// (ADR-0050 B3b, #511). Flips window.innerWidth across the 768px boundary so
// useIsMobile() picks the stacked-card renderer vs the wide table, then asserts
// the divergence contract: only one tree mounts, the rate (the primary figure)
// reads under the shared `inflation-rate-value` testid in each, and the mobile
// card's delete control clears the ≥44px tap floor.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { InflationRatesCard } from "@/components/common/InflationRatesCard";
import type { InflationRate } from "@/api/types";

const rates: InflationRate[] = [
  {
    id: "inf-1",
    household_id: "hh-1",
    year_month: "2026-05",
    rate: "3.5",
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
  server.use(http.get("/api/inflation-rates", () => HttpResponse.json(rates)));
  return renderWithProviders(<InflationRatesCard />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes the rate, meets the 44px tap floor", async () => {
  setViewport(500);
  renderCard();

  // Card renderer mounted, table renderer did not — one tree in the DOM.
  expect(await screen.findByTestId("inflation-rate-cards")).toBeInTheDocument();
  expect(screen.queryByTestId("inflation-rate-table")).not.toBeInTheDocument();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();

  // Rate (primary figure) reads, with the unit made explicit on the card.
  expect(screen.getByTestId("inflation-rate-value")).toHaveTextContent("3.50%");

  // A11y floor: the delete control is a ≥44px (size-11) tap target.
  expect(screen.getByRole("button", { name: "Delete" })).toHaveClass("size-11");
});

it("desktop: mounts the table renderer under the same primary-figure testid", async () => {
  setViewport(1280);
  renderCard();

  expect(await screen.findByTestId("inflation-rate-table")).toBeInTheDocument();
  expect(screen.getByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("inflation-rate-cards")).not.toBeInTheDocument();
  expect(screen.getByTestId("inflation-rate-value")).toHaveTextContent("3.50%");
});

it("desktop: Edit swaps the rate into an input and Save PATCHes the new value", async () => {
  setViewport(1280);
  let patched: { id: string; body: unknown } | null = null;
  server.use(
    http.patch("/api/inflation-rates/:id", async ({ params, request }) => {
      patched = { id: String(params.id), body: await request.json() };
      return HttpResponse.json({ ...rates[0], rate: "4.2" });
    }),
  );
  renderCard();

  await screen.findByTestId("inflation-rate-table");
  const row = within(screen.getByTestId("inflation-rate-row"));
  fireEvent.click(row.getByRole("button", { name: "Edit" }));

  // Rate becomes an editable input pre-filled with the current value (scoped to
  // the row — the add form shares the "Annual %" label).
  const input = row.getByLabelText("Annual %");
  expect(input).toHaveValue("3.5");
  fireEvent.change(input, { target: { value: "4.2" } });
  fireEvent.click(row.getByRole("button", { name: "Save" }));

  await waitFor(() => expect(patched).not.toBeNull());
  expect(patched).toEqual({ id: "inf-1", body: { rate: "4.2" } });
});

it("desktop: Cancel leaves the row unedited and fires no request", async () => {
  setViewport(1280);
  let patchCalls = 0;
  server.use(
    http.patch("/api/inflation-rates/:id", () => {
      patchCalls += 1;
      return HttpResponse.json(rates[0]);
    }),
  );
  renderCard();

  await screen.findByTestId("inflation-rate-table");
  const row = within(screen.getByTestId("inflation-rate-row"));
  fireEvent.click(row.getByRole("button", { name: "Edit" }));
  fireEvent.change(row.getByLabelText("Annual %"), { target: { value: "9.9" } });
  fireEvent.click(row.getByRole("button", { name: "Cancel" }));

  // Back to the read-only value; no PATCH went out.
  expect(screen.getByTestId("inflation-rate-value")).toHaveTextContent("3.50%");
  expect(patchCalls).toBe(0);
});

it("desktop: paginates at 12 rows per page", async () => {
  setViewport(1280);
  // 13 monthly figures → two pages (12 + 1).
  const many: InflationRate[] = Array.from({ length: 13 }, (_, i) => ({
    ...rates[0],
    id: `inf-${i}`,
    year_month: `2025-${String((i % 12) + 1).padStart(2, "0")}`,
  }));
  server.use(http.get("/api/inflation-rates", () => HttpResponse.json(many)));
  renderWithProviders(<InflationRatesCard />);

  await screen.findByTestId("inflation-rate-table");
  expect(screen.getAllByTestId("inflation-rate-row")).toHaveLength(12);

  // Jump to page 2 → the remaining single row.
  fireEvent.click(screen.getByText("2"));
  expect(screen.getAllByTestId("inflation-rate-row")).toHaveLength(1);
});

it("shows no pagination control for a single page", async () => {
  setViewport(1280);
  renderCard();
  await screen.findByTestId("inflation-rate-table");
  expect(screen.queryByTestId("pagination-next")).not.toBeInTheDocument();
});
