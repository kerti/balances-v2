// Renderer conformance for the Income mobile–web split (ADR-0050 B2b, #508).
// Flips window.innerWidth across the 768px boundary so useIsMobile() picks the
// card renderer vs the table renderer, then asserts the divergence contract:
// only one tree mounts, the SAME `income-*` data-testids resolve in both, the
// amount (the primary value) is readable in each, and the mobile ⋮ action
// clears the ≥44px tap floor. All row logic (edit/duplicate/delete dialogs,
// delete mutation) lives in the shared useIncomeRow container — the leaves are
// pure views — so behaviour can't fork per renderer.
import { it, expect, afterEach } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, waitFor } from "@testing-library/react";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { IncomeScreen } from "@/components/screens/IncomeScreen";
import type { Income } from "@/api/types";

const income: Income = {
  id: "inc-1",
  household_id: "hh-1",
  date: "2026-05-10",
  amount: "15000000",
  currency: "IDR",
  category: "salary",
  description: "May salary",
  ownership_type: "joint",
  sole_owner_user_id: null,
  regularity: "routine",
  created_by: null,
  created_at: "2026-05-10T00:00:00Z",
  updated_by: null,
  updated_at: "2026-05-10T00:00:00Z",
};

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

function renderScreen() {
  server.use(
    http.get("/api/income", () => HttpResponse.json([income])),
    http.get("/api/household/members", () => HttpResponse.json([])),
  );
  return renderWithProviders(<IncomeScreen />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes the amount, meets the 44px tap floor", async () => {
  setViewport(500);
  renderScreen();

  // Card renderer mounted, table renderer did not — one tree in the DOM.
  expect(await screen.findByTestId("income-card-list")).toBeInTheDocument();
  expect(screen.queryByTestId("income-table")).not.toBeInTheDocument();

  // Same testid contract as desktop; the amount (primary value) is present.
  expect(screen.getByTestId("income-amount")).toHaveTextContent(/15,000,000/);

  // A11y floor: the ⋮ action is a full ≥44px (size-11) tap target on the card.
  expect(screen.getByRole("button", { name: /income actions/i })).toHaveClass("size-11");
});

it("desktop: mounts the table renderer under the same amount testid", async () => {
  setViewport(1280);
  renderScreen();

  expect(await screen.findByTestId("income-table")).toBeInTheDocument();
  expect(screen.queryByTestId("income-card-list")).not.toBeInTheDocument();
  expect(screen.getByTestId("income-amount")).toHaveTextContent(/15,000,000/);

  // The desktop ⋮ stays the compact icon target (no mobile 44px bump), proving
  // the leaves really diverge rather than sharing one sized button.
  await waitFor(() =>
    expect(screen.getByRole("button", { name: /income actions/i })).not.toHaveClass("size-11"),
  );
});
