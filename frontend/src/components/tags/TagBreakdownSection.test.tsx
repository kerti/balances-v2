// Renderer conformance for the Tag breakdown mobile–web split (ADR-0050 B2c,
// #509). Flips window.innerWidth across the 768px boundary so useIsMobile()
// picks the stacked-card renderer vs the wide table, then asserts the
// divergence contract: only one tree mounts under the shared
// `tag-breakdown-<currency>` testid, the net value (the primary figure) reads
// in each, and the mobile card's pie-inclusion toggle clears the ≥44px tap
// floor. The container (TagsScreen) owns the query and the checked state, so
// the pie-inclusion behaviour can't fork per renderer.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { TagsScreen } from "@/components/screens/TagsScreen";
import type { Tag, TagBreakdownRow } from "@/api/types";

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

const rows: TagBreakdownRow[] = [
  { tag_id: "tag-1", group: "investment", currency: "IDR", total: "80000000" },
  { tag_id: "tag-1", group: "liability", currency: "IDR", total: "30000000" },
];

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderScreen() {
  server.use(
    http.get("/api/tags", () => HttpResponse.json([tag])),
    http.get("/api/tags/breakdown", () => HttpResponse.json(rows)),
  );
  return renderWithProviders(<TagsScreen />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes net, meets the 44px tap floor", async () => {
  setViewport(500);
  renderScreen();

  // Card renderer mounted, table renderer did not — one tree in the DOM under
  // the shared per-currency testid.
  expect(await screen.findByTestId("tag-breakdown-IDR")).toBeInTheDocument();
  expect(await screen.findByTestId("tag-breakdown-cards-IDR")).toBeInTheDocument();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();

  // Net (primary figure) reads: 80,000,000 − 30,000,000 = 50,000,000.
  expect(screen.getAllByTestId("tag-breakdown-net")[0]).toHaveTextContent(/50,000,000/);

  // A11y floor: the pie-inclusion toggle is a ≥44px tap target (min-h-11 label).
  const toggle = screen.getByLabelText("Retirement").closest("label");
  expect(toggle).toHaveClass("min-h-11");
});

it("desktop: mounts the table renderer under the same per-currency testid", async () => {
  setViewport(1280);
  renderScreen();

  expect(await screen.findByTestId("tag-breakdown-IDR")).toBeInTheDocument();
  expect(await screen.findByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("tag-breakdown-cards-IDR")).not.toBeInTheDocument();
});
