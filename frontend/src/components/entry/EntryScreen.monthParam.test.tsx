// ?month=YYYY-MM seeds the bulk-entry month (#792); anything invalid or in the
// future falls back to the current month.
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { thisYearMonth } from "@/lib/dateLimits";
import { formatYearMonth } from "@/lib/format";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { assetEntryConfig } from "@/components/entry/groups";

const monthLabel = (ym: string) => formatYearMonth(`${ym}-01T12:00:00Z`);

function renderAt(search: string) {
  server.use(
    http.get("/api/assets/snapshots/entry", () =>
      HttpResponse.json({ year_month: "2026-03", rows: [] }),
    ),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  return renderWithProviders(
    <MemoryRouter initialEntries={[`/assets/enter${search}`]}>
      <EntryScreen config={assetEntryConfig} />
    </MemoryRouter>,
  );
}

it("seeds the month from ?month= but leaves the statement date blank (#790)", async () => {
  renderAt("?month=2026-03");
  expect(await screen.findByTestId("asset-entry-month")).toHaveTextContent(monthLabel("2026-03"));
  expect(screen.getByLabelText(/statement date/i)).toHaveValue("");
});

it.each(["?month=2026-13", "?month=garbage", "?month=2999-01", ""])(
  "falls back to the current month for %j",
  async (search) => {
    renderAt(search);
    expect(await screen.findByTestId("asset-entry-month")).toHaveTextContent(
      monthLabel(thisYearMonth()),
    );
  },
);
