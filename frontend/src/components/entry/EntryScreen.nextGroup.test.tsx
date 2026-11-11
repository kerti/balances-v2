// After a bulk save, the "Saved N" toast offers the next group that still has
// stale positions that month (ADR-0058 §6, #805). "Stale" is the month report's
// stale_positions, the same list the dashboard warns from.
import { it, expect, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { assetEntryConfig } from "@/components/entry/groups";
import { routes } from "@/lib/routes";

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const assetList = {
  year_month: "2026-05",
  rows: [
    {
      asset_id: "a1",
      display_name: "Checking",
      subtype: "bank_account",
      currency: "IDR",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_amount: "1000",
      carried_from: "2026-04",
    },
  ],
};

function stale(group: string, subtype: string, id: string) {
  return { position_id: id, name: id, group, subtype, last_month: "2026-03-01T00:00:00Z" };
}

function mockServer(stalePositions: unknown[]) {
  server.use(
    http.get("/api/assets/snapshots/entry", () => HttpResponse.json(assetList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
    http.post("/api/assets/snapshots/bulk", () => HttpResponse.json({ written: 1, replaced: 0 })),
    http.get("/api/reports", () =>
      HttpResponse.json([
        { year_month: "2026-04-01T00:00:00Z", stale_positions: [] },
        { year_month: "2026-05-01T00:00:00Z", stale_positions: stalePositions },
      ]),
    ),
  );
}

function Where() {
  const loc = useLocation();
  return <div data-testid="where">{loc.pathname + loc.search}</div>;
}

async function saveAssets() {
  renderWithProviders(
    <MemoryRouter initialEntries={["/assets/enter?month=2026-05"]}>
      <Routes>
        <Route path="/assets/enter" element={<EntryScreen config={assetEntryConfig} />} />
        <Route path="*" element={<Where />} />
      </Routes>
    </MemoryRouter>,
  );
  const user = userEvent.setup();
  const input = await screen.findByTestId("asset-entry-amount-a1");
  await user.clear(input);
  await user.type(input, "2000");
  await user.click(screen.getByTestId("asset-entry-save"));
  await screen.findByTestId("where");
  return user;
}

// covers: INV-SNAPSHOTS-16
it("offers the next group with stale positions on the saved toast and links to it", async () => {
  mockServer([
    stale("asset", "bank_account", "still-stale-asset"),
    stale("investment", "stock", "s1"),
    stale("investment", "gold", "s2"),
  ]);
  const user = await saveAssets();
  void user;

  // Saving still navigates back.
  expect(screen.getByTestId("where")).toHaveTextContent(routes.assets);
  const toasts = raisedToasts(mockedToast);
  expect(toasts).toHaveLength(1);
  expect(toasts[0].message).toBe("Saved 1");
  // The current group (Assets) is never offered, even though it is still stale.
  expect(toasts[0].options.action?.label).toBe("Next: Investments (2) →");
  expect(toasts[0].options).toMatchObject({ duration: 8000 });

  toasts[0].options.action!.onClick();
  await waitFor(() =>
    expect(screen.getByTestId("where")).toHaveTextContent(
      `${routes.investmentsEnterPrices}?month=2026-05`,
    ),
  );
});

// covers: INV-SNAPSHOTS-16
it("carries no next action when nothing else is stale that month", async () => {
  mockServer([stale("asset", "bank_account", "still-stale-asset")]);
  await saveAssets();

  const toasts = raisedToasts(mockedToast);
  expect(toasts).toHaveLength(1);
  expect(toasts[0].options.action).toBeUndefined();
  expect(toasts[0].options).toMatchObject({ duration: 4000 });
});

it("keeps the plain toast when the report cannot be read", async () => {
  mockServer([]);
  server.use(http.get("/api/reports", () => new HttpResponse(null, { status: 500 })));
  await saveAssets();

  const toasts = raisedToasts(mockedToast);
  expect(toasts).toHaveLength(1);
  expect(toasts[0].options.action).toBeUndefined();
});
