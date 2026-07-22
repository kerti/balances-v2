// Mobile-renderer conformance for the qty×price bulk monthly-entry view
// (ADR-0050 S2, #505). The qty×price twin of the amount-only mobile test (#502):
// flips window.innerWidth below the 768px boundary so useIsMobile() picks
// EntryRowMobile, then asserts the divergence contract for the two-tab-stop
// shape — quantity and price each stack on their own full-width ≥44px line (not
// the cramped desktop w-28 columns), the computed value shows in the footer marked
// "= …" (mobile has no desktop column header to label the total), and the
// container-only "half a pair isn't saveable" dirty rule survives the renderer
// swap untouched. The SAME data-testids resolve as on desktop, so the deep
// per-shape assertions stay in the desktop twin (EntryScreen.investment.test.tsx).
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { MemoryRouter } from "react-router";
import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { EntryScreen } from "@/components/entry/EntryScreen";
import { investmentEntryConfig } from "@/components/entry/groups";

const entryList = {
  year_month: "2026-05",
  rows: [
    {
      investment_id: "s1",
      display_name: "BBCA",
      currency: "IDR",
      subtype: "stock",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_quantity: "10",
      prefill_price: "1500",
      carried_from: "2026-04",
    },
    {
      investment_id: "g1",
      display_name: "Antam 100g",
      currency: "IDR",
      subtype: "gold",
      ownership_type: "joint",
      sole_owner_user_id: null,
      prefill_quantity: null,
      prefill_price: null,
      carried_from: null,
    },
  ],
};

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderScreen() {
  return renderWithProviders(
    <MemoryRouter>
      <EntryScreen config={investmentEntryConfig} />
    </MemoryRouter>,
  );
}

// covers: INV-SNAPSHOTS-07
// covers: INV-SNAPSHOTS-08
it("mobile: stacks quantity + price as full-width ≥44px lines with the value marked computed", async () => {
  setViewport(500);
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();

  // Same testid contract as desktop, carry-forward prefill intact on both tab-stops.
  const qty = await screen.findByTestId("investment-entry-quantity-s1");
  const price = screen.getByTestId("investment-entry-price-s1");
  expect(qty).toHaveValue("10");
  expect(price).toHaveValue("1500");
  // A11y floor: each tab-stop is a full-width ≥44px (h-11) target — the mobile
  // renderer, not the cramped desktop w-28 columns.
  expect(qty).toHaveClass("h-11");
  expect(qty).not.toHaveClass("w-28");
  expect(price).toHaveClass("h-11");
  expect(price).not.toHaveClass("w-28");
  // Mobile labels each field inline within its own row (desktop labels them once
  // per group header instead); the inline <Label>s wire to the same inputs.
  const stockRow = screen.getByTestId("investment-entry-row-s1");
  expect(within(stockRow).getByText("Quantity")).toBeInTheDocument();
  expect(within(stockRow).getByText("Price per unit")).toBeInTheDocument();
  expect(within(stockRow).getByLabelText("Quantity")).toBe(qty);
  expect(within(stockRow).getByLabelText("Price per unit")).toBe(price);
  // The computed value (10 × 1500 = 15000) shows in the footer, marked "=" so a
  // mobile user reads it as derived (no desktop column header here).
  expect(screen.getByTestId("investment-entry-value-s1")).toHaveTextContent(/^=.*15/);
  // The fresh gold's incomplete pair shows the bare placeholder — no "=".
  const freshValue = screen.getByTestId("investment-entry-value-g1");
  expect(freshValue).toHaveTextContent("—");
  expect(freshValue).not.toHaveTextContent("=");
});

// covers: INV-SNAPSHOTS-06
it("mobile: a half-filled qty×price pair stays not-dirty — the container rule survives the renderer swap", async () => {
  setViewport(500);
  server.use(
    http.get("/api/investments/snapshots/entry", () => HttpResponse.json(entryList)),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  renderScreen();
  const user = userEvent.setup();

  // Type only the quantity for the fresh gold — no price. Incomplete pair: the
  // "one of qty/price typed → not yet saveable" rule lives in EntryScreen, not
  // the renderer, so the mobile swap must not change it.
  const goldQty = await screen.findByTestId("investment-entry-quantity-g1");
  await user.type(goldQty, "5");

  expect(screen.getByTestId("investment-entry-dirty-count")).toHaveTextContent(/0/);
  expect(screen.getByTestId("investment-entry-save")).toBeDisabled();
  // Still no "=" — the derived value stays a placeholder until the pair completes.
  expect(screen.getByTestId("investment-entry-value-g1")).toHaveTextContent("—");
});
