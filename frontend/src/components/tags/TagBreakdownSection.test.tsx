// Renderer conformance for the Tag breakdown mobile–web split (ADR-0050 B2c,
// #509). Flips window.innerWidth across the 768px boundary so useIsMobile()
// picks the stacked-card renderer vs the wide table, then asserts the
// divergence contract: only one tree mounts under the shared
// `tag-breakdown` testid, the net value (the primary figure) reads
// in each, and the mobile card's pie-inclusion toggle clears the ≥44px tap
// floor. The container (TagsScreen) owns the query and the checked state, so
// the pie-inclusion behaviour can't fork per renderer.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, within } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { TagsScreen } from "@/components/screens/TagsScreen";
import type { Tag, TagBreakdown } from "@/api/types";

const tag: Tag = {
  id: "tag-1",
  household_id: "hh-1",
  name: "Retirement",
  color: "#2563eb",
  created_by: null,
  created_at: "2026-05-10T00:00:00Z",
  updated_by: null,
  updated_at: "2026-05-10T00:00:00Z",
  deleted_at: null,
};

function breakdown(over: Partial<TagBreakdown> = {}): TagBreakdown {
  return {
    reporting_currency: "IDR",
    year_month: "2026-11-01T00:00:00Z",
    tags: [{ tag_id: "tag-1", holdings: "80000000", liabilities: "30000000" }],
    missing_fx: [],
    fx_rates_used: {},
    ...over,
  };
}

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderScreen(data: TagBreakdown = breakdown()) {
  server.use(
    http.get("/api/tags", () => HttpResponse.json([tag])),
    http.get("/api/tags/breakdown", () => HttpResponse.json(data)),
  );
  return renderWithProviders(<TagsScreen />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes net, meets the 44px tap floor", async () => {
  setViewport(500);
  renderScreen();

  // Card renderer mounted, table renderer did not — one tree in the DOM under
  // the shared testid.
  expect(await screen.findByTestId("tag-breakdown")).toBeInTheDocument();
  expect(await screen.findByTestId("tag-breakdown-cards")).toBeInTheDocument();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();

  // Net (primary figure) reads: 80,000,000 − 30,000,000 = 50,000,000.
  expect(screen.getAllByTestId("tag-breakdown-net")[0]).toHaveTextContent(/50,000,000/);

  // A11y floor: the pie-inclusion toggle is a ≥44px tap target (min-h-11 label).
  const toggle = screen.getByLabelText("Retirement").closest("label");
  expect(toggle).toHaveClass("min-h-11");
});

it("desktop: mounts the table renderer under the same testid", async () => {
  setViewport(1280);
  renderScreen();

  expect(await screen.findByTestId("tag-breakdown")).toBeInTheDocument();
  expect(await screen.findByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("tag-breakdown-cards")).not.toBeInTheDocument();
});

it("converted: a tag holding several currencies renders one row and one slice", async () => {
  setViewport(1280);
  // 80,000,000 is already the converted sum of the tag's IDR + USD positions.
  renderScreen(
    breakdown({
      tags: [{ tag_id: "tag-1", holdings: "80000000", liabilities: "0" }],
      fx_rates_used: { USD: "16000" },
    }),
  );

  const section = await screen.findByTestId("tag-breakdown");
  // header + tag row + total row
  expect(within(section).getAllByRole("row")).toHaveLength(3);
  expect(within(section).getAllByLabelText("Retirement")).toHaveLength(1);
  expect(within(section).getAllByText(/80,000,000/).length).toBeGreaterThan(0);
});

it("shows the rates line only when rates were used", async () => {
  setViewport(1280);
  renderScreen(breakdown({ fx_rates_used: { USD: "16000" } }));
  const line = await screen.findByTestId("tag-breakdown-rates");
  expect(line).toHaveTextContent(/1 USD = .*16,000.* IDR/);
});

it("omits the rates line when no rate was used", async () => {
  setViewport(1280);
  renderScreen();
  await screen.findByTestId("tag-breakdown");
  expect(screen.queryByTestId("tag-breakdown-rates")).not.toBeInTheDocument();
});

it("names the currency of a position left out for want of a rate", async () => {
  setViewport(1280);
  renderScreen(
    breakdown({ missing_fx: [{ position_id: "p1", currency: "EUR", tag_id: "tag-1" }] }),
  );
  const warning = await screen.findByTestId("missing-fx-warning");
  expect(warning).toHaveTextContent(/EUR/);
});
