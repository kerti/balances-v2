// The "Same" column header only appears over a group that has a row to tick
// (#939), and the title names the picked month while rows read "Recorded" (#940).
import { it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { formatYearMonth } from "@/lib/format";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { assetEntryConfig } from "@/components/entry/groups";

const base = { currency: "IDR", ownership_type: "joint", sole_owner_user_id: null };

const recorded = {
  ...base,
  asset_id: "r1",
  display_name: "Recorded one",
  subtype: "bank_account",
  prefill_amount: "900000",
  carried_from: "2026-05",
};
const carried = {
  ...base,
  asset_id: "c1",
  display_name: "Carried one",
  subtype: "cash",
  prefill_amount: "700000",
  carried_from: "2026-04",
};

function renderRows(rows: unknown[]) {
  server.use(
    http.get("/api/assets/snapshots/entry", () =>
      HttpResponse.json({ year_month: "2026-05", rows }),
    ),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  return renderWithProviders(
    <MemoryRouter initialEntries={["/?month=2026-05"]}>
      <EntryScreen config={assetEntryConfig} />
    </MemoryRouter>,
  );
}

it("shows no Same header when no row in the group can be ticked", async () => {
  renderRows([recorded]);
  await screen.findByTestId("asset-entry-row-r1");
  expect(screen.queryByText("Same")).not.toBeInTheDocument();
});

it("shows the Same header only over the groups that have a carried row", async () => {
  renderRows([recorded, carried]);
  await screen.findByTestId("asset-entry-row-c1");
  const headers = screen.getAllByText("Same");
  expect(headers).toHaveLength(1);
  expect(screen.getByTestId("asset-entry-group-cash")).toContainElement(headers[0]);
});

it("keeps the Same header while the carried row is being edited", async () => {
  renderRows([carried]);
  const user = userEvent.setup();
  const input = await screen.findByTestId("asset-entry-amount-c1");
  await user.clear(input);
  await user.type(input, "800000");
  expect(screen.getByText("Same")).toBeInTheDocument();
});

it("titles the page with the picked month and labels the row value Recorded", async () => {
  renderRows([recorded]);
  const month = formatYearMonth("2026-05-01T12:00:00Z");
  expect(await screen.findByTestId("asset-entry-current-r1")).toHaveTextContent(
    /^Recorded: .*900,000/,
  );
  expect(screen.getByRole("heading", { name: `Enter balances for ${month}` })).toBeVisible();
});

it("the title follows the month picker", async () => {
  renderRows([recorded]);
  const user = userEvent.setup();
  await screen.findByTestId("asset-entry-row-r1");
  await user.click(screen.getByTestId("month-picker-prev"));
  const april = formatYearMonth("2026-04-01T12:00:00Z");
  expect(screen.getByRole("heading", { name: `Enter balances for ${april}` })).toBeVisible();
});
