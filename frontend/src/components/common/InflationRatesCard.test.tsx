// Renderer conformance for the Settings ▸ Inflation Rates mobile–web split
// (ADR-0050 B3b, #511). Flips window.innerWidth across the 768px boundary so
// useIsMobile() picks the stacked-card renderer vs the wide table, then asserts
// the divergence contract: only one tree mounts, the rate (the primary figure)
// reads under the shared `inflation-rate-value` testid in each, and the mobile
// card's delete control clears the ≥44px tap floor.
import { it, expect, afterEach, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
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

vi.mock("sonner", async () => (await import("@/test/sonnerMock")).sonnerMock());
const mockedToast = toast as unknown as Parameters<typeof raisedToasts>[0];

const originalWidth = window.innerWidth;

function setViewport(width: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
}

afterEach(() => {
  setViewport(originalWidth);
});

// Open a row's ⋯ menu and pick an item (the only way to Edit or Delete a row).
function pickRowAction(scope: ReturnType<typeof within>, item: "Edit" | "Delete") {
  fireEvent.click(scope.getByRole("button", { name: "Inflation rate actions" }));
  fireEvent.click(screen.getByRole("menuitem", { name: item }));
}

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

  // A11y floor: the ⋯ menu trigger is a ≥44px (size-11) tap target.
  expect(screen.getByRole("button", { name: "Inflation rate actions" })).toHaveClass("size-11");
});

it("desktop: mounts the table renderer under the same primary-figure testid", async () => {
  setViewport(1280);
  renderCard();

  expect(await screen.findByTestId("inflation-rate-table")).toBeInTheDocument();
  expect(screen.getByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("inflation-rate-cards")).not.toBeInTheDocument();
  expect(screen.getByTestId("inflation-rate-value")).toHaveTextContent("3.50%");
});

it("desktop: Edit opens the dialog with the month locked and Save PATCHes the new rate", async () => {
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
  pickRowAction(within(screen.getByTestId("inflation-rate-row")), "Edit");

  const dialog = within(await screen.findByRole("dialog"));
  expect(dialog.getByLabelText("Month")).toBeDisabled();
  const input = dialog.getByLabelText("Annual %");
  expect(input).toHaveValue("3.5");
  fireEvent.change(input, { target: { value: "4.2" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));

  await waitFor(() => expect(patched).not.toBeNull());
  expect(patched).toEqual({ id: "inf-1", body: { rate: "4.2" } });
});

it("desktop: Cancel closes the edit dialog and fires no request", async () => {
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
  pickRowAction(within(screen.getByTestId("inflation-rate-row")), "Edit");
  const dialog = within(await screen.findByRole("dialog"));
  fireEvent.change(dialog.getByLabelText("Annual %"), { target: { value: "9.9" } });
  fireEvent.click(dialog.getByRole("button", { name: "Cancel" }));

  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByTestId("inflation-rate-value")).toHaveTextContent("3.50%");
  expect(patchCalls).toBe(0);
});

it("New inflation rate opens a dialog that POSTs the month and rate", async () => {
  setViewport(1280);
  let posted: unknown = null;
  server.use(
    http.post("/api/inflation-rates", async ({ request }) => {
      posted = await request.json();
      return HttpResponse.json(rates[0]);
    }),
  );
  renderCard();
  await screen.findByTestId("inflation-rate-table");

  // The old inline form is gone: nothing to type into until the dialog opens.
  expect(screen.queryByLabelText("Annual %")).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "New inflation rate" }));

  const dialog = within(await screen.findByRole("dialog"));
  const add = dialog.getByRole("button", { name: "Add rate" });
  expect(add).toBeDisabled();
  await userEvent.click(dialog.getByLabelText("Month"));
  await userEvent.click(await screen.findByTestId("month-picker-cell-2026-02"));
  fireEvent.change(dialog.getByLabelText("Annual %"), { target: { value: "-1.5" } });
  expect(add).toBeEnabled();
  fireEvent.click(add);

  await waitFor(() => expect(posted).not.toBeNull());
  expect(posted).toEqual({ year_month: "2026-02", rate: "-1.5" });
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

it("mobile: Edit from the card menu opens the dialog and Save PATCHes", async () => {
  setViewport(500);
  let patched: { id: string; body: unknown } | null = null;
  server.use(
    http.patch("/api/inflation-rates/:id", async ({ params, request }) => {
      patched = { id: String(params.id), body: await request.json() };
      return HttpResponse.json({ ...rates[0], rate: "4.2" });
    }),
  );
  renderCard();

  await screen.findByTestId("inflation-rate-cards");
  pickRowAction(within(screen.getByTestId("inflation-rate-row")), "Edit");

  const dialog = within(await screen.findByRole("dialog"));
  fireEvent.change(dialog.getByLabelText("Annual %"), { target: { value: "4.2" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));

  await waitFor(() => expect(patched).not.toBeNull());
  expect(patched).toEqual({ id: "inf-1", body: { rate: "4.2" } });
});

// covers: INV-PRESENTATION-17
it("desktop: trash deletes at once with no dialog, raising one Undo toast", async () => {
  setViewport(1280);
  let deletes = 0;
  let undos = 0;
  server.use(
    http.delete("/api/inflation-rates/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
    http.post("/api/inflation-rates/:id/undo-delete", () => {
      undos += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderCard();
  await screen.findByTestId("inflation-rate-table");

  pickRowAction(within(screen.getByTestId("inflation-rate-row")), "Delete");
  await waitFor(() => expect(deletes).toBe(1));
  expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();

  const raised = raisedToasts(mockedToast);
  expect(raised).toHaveLength(1);
  expect(raised[0].options.duration).toBe(8000);
  expect(raised[0].options.action?.label).toBe("Undo");

  raised[0].options.action?.onClick();
  await waitFor(() => expect(undos).toBe(1));
});

// covers: INV-PRESENTATION-17
it("mobile: the card's trash also deletes at once with no dialog", async () => {
  setViewport(500);
  let deletes = 0;
  server.use(
    http.delete("/api/inflation-rates/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderCard();
  await screen.findByTestId("inflation-rate-cards");

  pickRowAction(within(screen.getByTestId("inflation-rate-row")), "Delete");
  await waitFor(() => expect(deletes).toBe(1));
  expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  expect(raisedToasts(mockedToast)).toHaveLength(1);
});
