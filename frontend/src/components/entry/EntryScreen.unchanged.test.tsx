// "Unchanged" rows in bulk monthly entry (ADR-0046 amendment, #789). A carried
// row can be marked to save its prefill as this month's Snapshot; untouched rows
// still write nothing; a month that already has a value shows a muted
// "Current: …" with no same-as checkbox. Covered for all three shapes (amount,
// quantity × price, accrued) plus the mobile renderer.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import {
  assetEntryConfig,
  investmentEntryConfig,
  investmentAccruedEntryConfig,
} from "@/components/entry/groups";

const base = {
  currency: "IDR",
  ownership_type: "joint",
  sole_owner_user_id: null,
};

const assetList = {
  year_month: "2026-05",
  rows: [
    {
      ...base,
      asset_id: "a1",
      display_name: "Checking",
      subtype: "bank_account",
      prefill_amount: "5000000.1634",
      carried_from: "2026-04",
    },
    {
      ...base,
      asset_id: "a2",
      display_name: "Savings",
      subtype: "bank_account",
      prefill_amount: "700000",
      carried_from: "2026-04",
    },
    {
      ...base,
      asset_id: "a3",
      display_name: "Already entered",
      subtype: "bank_account",
      prefill_amount: "900000",
      carried_from: "2026-05",
    },
    {
      ...base,
      asset_id: "a4",
      display_name: "Brand new",
      subtype: "bank_account",
      prefill_amount: null,
      carried_from: null,
    },
  ],
};

const stockList = {
  year_month: "2026-05",
  rows: [
    {
      ...base,
      investment_id: "s1",
      display_name: "BBCA",
      subtype: "stock",
      prefill_quantity: "10",
      prefill_price: "1500",
      carried_from: "2026-04",
    },
  ],
};

const bondList = {
  year_month: "2026-05",
  rows: [
    {
      ...base,
      investment_id: "b1",
      display_name: "FR Series Bond",
      subtype: "bond",
      coupon_disposition: "pays_out",
      prefill_amount: "50250000",
      prefill_accrued_interest: "250000",
      carried_from: "2026-04",
    },
  ],
};

const originalWidth = window.innerWidth;
afterEach(() => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: originalWidth });
});

function mockAssets(onPost?: (body: unknown) => void) {
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(assetList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/assets/snapshots/bulk", async ({ request }) => {
      onPost?.(await request.json());
      return HttpResponse.json({ written: 1 });
    }),
  );
}

function renderScreen(config: Parameters<typeof EntryScreen>[0]["config"]) {
  return renderWithProviders(
    <MemoryRouter initialEntries={["/?month=2026-05"]}>
      <EntryScreen config={config} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-14
it("amount: Unchanged saves the stored prefill, shows 'Same as', and can be undone", async () => {
  let posted: { rows: Array<Record<string, string>> } | null = null;
  mockAssets((b) => (posted = b as typeof posted));
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  await user.click(await screen.findByTestId("asset-entry-same-a1"));
  expect(screen.getByTestId("asset-entry-unchanged-a1")).toHaveTextContent(/^Same as .*2026.*✓$/);
  // Not a change: no dot, no "changed" tick; the marked tally ticks instead.
  expect(screen.queryByTestId("asset-entry-dirty-a1")).not.toBeInTheDocument();
  expect(screen.getByTestId("asset-entry-dirty-count")).toHaveTextContent("0 changed");
  expect(screen.getByTestId("asset-entry-unchanged-count")).toHaveTextContent("1 unchanged");

  // Undo before saving drops it again.
  await user.click(screen.getByTestId("asset-entry-undo-a1"));
  expect(screen.queryByTestId("asset-entry-unchanged-a1")).not.toBeInTheDocument();
  expect(screen.getByTestId("asset-entry-save")).toBeDisabled();

  await user.click(screen.getByTestId("asset-entry-same-a1"));
  await user.click(screen.getByTestId("asset-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  // Only the marked row, with the stored (unrounded) value.
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ asset_id: "a1", amount: "5000000.1634" });
});

// covers: INV-SNAPSHOTS-14
// covers: INV-SNAPSHOTS-06
it("amount: untouched rows write nothing; editing a marked row makes it a plain change", async () => {
  let posted: { rows: Array<Record<string, string>> } | null = null;
  mockAssets((b) => (posted = b as typeof posted));
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  await screen.findByTestId("asset-entry-row-a2");
  const save = screen.getByTestId("asset-entry-save");
  expect(save).toBeDisabled();

  await user.click(screen.getByTestId("asset-entry-same-a2"));
  const input = screen.getByTestId("asset-entry-amount-a2");
  await user.clear(input);
  await user.type(input, "800000");
  expect(screen.queryByTestId("asset-entry-unchanged-a2")).not.toBeInTheDocument();
  expect(screen.getByTestId("asset-entry-dirty-count")).toHaveTextContent("1 changed");
  expect(screen.queryByTestId("asset-entry-unchanged-count")).not.toBeInTheDocument();

  await user.click(save);
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({ asset_id: "a2", amount: "800000" });
});

// covers: INV-SNAPSHOTS-14
it("the header checkbox ticks and unticks every markable row, with an indeterminate middle", async () => {
  let posted: { rows: Array<Record<string, string>> } | null = null;
  mockAssets((b) => (posted = b as typeof posted));
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  // a1 + a2 are carried; a3 already has this month's value; a4 has no history.
  await screen.findByTestId("asset-entry-row-a2");
  const all = screen.getByTestId("asset-entry-same-all-bank_account") as HTMLInputElement;
  expect(all.checked).toBe(false);
  expect(all.indeterminate).toBe(false);
  // Rows that can't be marked have no checkbox (an empty cell).
  expect(screen.queryByTestId("asset-entry-same-a3")).not.toBeInTheDocument();
  expect(screen.queryByTestId("asset-entry-same-a4")).not.toBeInTheDocument();

  await user.click(screen.getByTestId("asset-entry-same-a1"));
  expect(all.indeterminate).toBe(true);

  await user.click(all);
  expect(all.checked).toBe(true);
  expect(all.indeterminate).toBe(false);
  expect(screen.getByTestId("asset-entry-same-a2")).toBeChecked();
  expect(screen.getByTestId("asset-entry-unchanged-count")).toHaveTextContent("2 unchanged");

  await user.click(screen.getByTestId("asset-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows.map((r) => r.asset_id).sort()).toEqual(["a1", "a2"]);

  await user.click(all);
  expect(screen.getByTestId("asset-entry-same-a1")).not.toBeChecked();
  expect(screen.getByTestId("asset-entry-save")).toBeDisabled();
});

// covers: INV-SNAPSHOTS-14
it("the row checkbox is named '{name}: same as {month}' and unticking drops the mark", async () => {
  mockAssets();
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  const box = await screen.findByTestId("asset-entry-same-a1");
  expect(box).toHaveAccessibleName(/^Checking: same as .*2026/);
  await user.click(box);
  expect(box).toBeChecked();
  await user.click(box);
  expect(box).not.toBeChecked();
  expect(screen.queryByTestId("asset-entry-unchanged-a1")).not.toBeInTheDocument();
});

it("a month that already has a value shows a muted 'Current: …' and no same-as checkbox", async () => {
  mockAssets();
  renderScreen(assetEntryConfig);

  const current = await screen.findByTestId("asset-entry-current-a3");
  expect(current).toHaveTextContent(/^Current: .*900,000/);
  expect(current).not.toHaveClass("text-warning");
  expect(screen.queryByTestId("asset-entry-same-a3")).not.toBeInTheDocument();
  expect(screen.queryByTestId("asset-entry-same-a4")).not.toBeInTheDocument();
  // Existing values are calm: no dot until the row is changed.
  expect(screen.queryByTestId("asset-entry-dirty-a3")).not.toBeInTheDocument();
});

// covers: INV-SNAPSHOTS-14
it("quantity × price: Unchanged sends the stored quantity and price", async () => {
  let posted: { rows: Array<Record<string, string>> } | null = null;
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(stockList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen(investmentEntryConfig);
  const user = userEvent.setup();

  await user.click(await screen.findByTestId("investment-entry-same-s1"));
  expect(screen.getByTestId("investment-entry-unchanged-s1")).toBeInTheDocument();
  await user.click(screen.getByTestId("investment-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows).toHaveLength(1);
  expect(posted!.rows[0]).toMatchObject({
    investment_id: "s1",
    quantity: "10",
    price_per_unit: "1500",
  });
});

// covers: INV-SNAPSHOTS-14
it("accrued: Unchanged sends the stored total value and accrued interest", async () => {
  let posted: { rows: Array<Record<string, string>> } | null = null;
  server.use(
    http.get("/api/investments/snapshots/accrued/entry", () => HttpResponse.json(bondList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/investments/snapshots/accrued/bulk", async ({ request }) => {
      posted = (await request.json()) as typeof posted;
      return HttpResponse.json({ written: 1 });
    }),
  );
  renderScreen(investmentAccruedEntryConfig);
  const user = userEvent.setup();

  await user.click(await screen.findByTestId("investment-accrued-entry-same-b1"));
  await user.click(screen.getByTestId("investment-accrued-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows[0]).toMatchObject({
    investment_id: "b1",
    amount: "50250000",
    accrued_interest: "250000",
  });
});

// covers: INV-SNAPSHOTS-14
it("mobile: the row checkbox has a 44px hit area, and an 'All same' toolbar checkbox ticks all", async () => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: 500 });
  let posted: { rows: Array<Record<string, string>> } | null = null;
  mockAssets((b) => (posted = b as typeof posted));
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  const box = await screen.findByTestId("asset-entry-same-a2");
  expect(box.closest("label")).toHaveClass("h-11", "min-w-11");
  await user.click(box);
  expect(screen.getByTestId("asset-entry-unchanged-a2")).toBeInTheDocument();

  const all = screen.getByTestId("asset-entry-same-all") as HTMLInputElement;
  expect(all.indeterminate).toBe(true);
  await user.click(all);
  expect(screen.getByTestId("asset-entry-same-a1")).toBeChecked();

  await user.click(screen.getByTestId("asset-entry-save"));
  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted!.rows.map((r) => r.asset_id).sort()).toEqual(["a1", "a2"]);
});

it("mobile: Undo renders inline only when the row is dirty or marked, with no reserved footer", async () => {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: 500 });
  mockAssets();
  renderScreen(assetEntryConfig);
  const user = userEvent.setup();

  const row = await screen.findByTestId("asset-entry-row-a2");
  expect(screen.queryByTestId("asset-entry-undo-a2")).not.toBeInTheDocument();
  // Amount-only has no derived footer, so the row ends at the input line.
  expect(row.children).toHaveLength(2);

  await user.click(screen.getByTestId("asset-entry-same-a2"));
  const undo = screen.getByTestId("asset-entry-undo-a2");
  expect(undo).toHaveClass("size-11");
  expect(undo.parentElement).toContainElement(screen.getByTestId("asset-entry-amount-a2"));
  expect(row.children).toHaveLength(2);
  await user.click(undo);
  expect(screen.queryByTestId("asset-entry-undo-a2")).not.toBeInTheDocument();
});

it("the month and statement-date inputs stack below md and sit in two columns from md", async () => {
  mockAssets();
  renderScreen(assetEntryConfig);
  const grid = (await screen.findByTestId("asset-entry-asof")).closest(".grid.items-end");
  expect(grid).toHaveClass("md:grid-cols-2");
  expect(grid).not.toHaveClass("grid-cols-2");
});
