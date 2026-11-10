// Renderer conformance for the Settings ▸ Exchange Rates mobile–web split
// (ADR-0050 B3b, #511). Flips window.innerWidth across the 768px boundary so
// useIsMobile() picks the stacked-card renderer vs the wide table, then asserts
// the divergence contract: only one tree mounts, the rate (the primary figure)
// reads under the shared `fx-rate-value` testid in each, and the mobile card's
// delete control clears the ≥44px tap floor.
import { it, expect, afterEach, vi } from "vitest";
import { http, HttpResponse } from "msw";
import { screen, fireEvent, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { server } from "@/test/server";
import { raisedToasts } from "@/test/sonnerMock";
import userEvent from "@testing-library/user-event";
import { renderWithProviders } from "@/test/renderWithProviders";
import { FxRatesCard } from "@/components/common/FxRatesCard";
import type { FxRate } from "@/api/types";
import type { Me } from "@/hooks/useSession";

const me: Me = {
  id: "u1",
  household_id: "hh-1",
  household_display_name: "Test Household",
  display_name: "Pat Owner",
  nickname: null,
  email: "pat@example.test",
  picture_url: null,
  locale: "en-GB",
  theme: "system",
  carryover_date_mode: "month_end",
  time_zone: "UTC",
  reporting_currency: "IDR",
  multi_currency_enabled: true,
  assumed_annual_inflation: "3.5",
  is_founder: true,
};

const rates: FxRate[] = [
  {
    id: "fx-1",
    household_id: "hh-1",
    year_month: "2026-05",
    currency: "USD",
    rate: "16250",
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
  fireEvent.click(scope.getByRole("button", { name: "Exchange rate actions" }));
  fireEvent.click(screen.getByRole("menuitem", { name: item }));
}

// The add form lives in a dialog now: open it from the card toolbar.
async function openNewRate() {
  fireEvent.click(screen.getByRole("button", { name: "New exchange rate" }));
  return within(await screen.findByRole("dialog"));
}

function renderCard() {
  server.use(
    http.get("/api/fx-rates", () => HttpResponse.json(rates)),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
  return renderWithProviders(<FxRatesCard />);
}

// covers: INV-PRESENTATION-08
it("mobile: mounts the card renderer, promotes the rate, meets the 44px tap floor", async () => {
  setViewport(500);
  renderCard();

  // Card renderer mounted, table renderer did not — one tree in the DOM.
  expect(await screen.findByTestId("fx-rate-cards")).toBeInTheDocument();
  expect(screen.queryByTestId("fx-rate-table")).not.toBeInTheDocument();
  expect(screen.queryByRole("table")).not.toBeInTheDocument();

  // Rate (primary figure) reads — as an equation binding it to the reporting
  // currency, so "16250" can't misread as "16250 USD".
  expect(await screen.findByText("1 USD = 16,250 IDR")).toBeInTheDocument();
  expect(screen.getByTestId("fx-rate-value")).toHaveTextContent("16,250");

  // A11y floor: the ⋯ menu trigger is a ≥44px (size-11) tap target.
  expect(screen.getByRole("button", { name: "Exchange rate actions" })).toHaveClass("size-11");
});

it("desktop: mounts the table renderer under the same primary-figure testid", async () => {
  setViewport(1280);
  renderCard();

  expect(await screen.findByTestId("fx-rate-table")).toBeInTheDocument();
  expect(screen.getByRole("table")).toBeInTheDocument();
  expect(screen.queryByTestId("fx-rate-cards")).not.toBeInTheDocument();
  expect(screen.getByTestId("fx-rate-value")).toHaveTextContent("16,250");
  expect(await screen.findByText("USD → IDR")).toBeInTheDocument();
});

it("new-rate dialog: shows a live direction hint once a currency code is entered", async () => {
  setViewport(1280);
  renderCard();

  // Wait for the session so the reporting currency (the counterpart) is known.
  await screen.findByTestId("fx-rate-table");
  const dialog = await openNewRate();

  // No code yet → no hint.
  expect(screen.queryByTestId("fx-rate-hint")).not.toBeInTheDocument();

  // Typing a 3-letter code spells out the direction + counterpart, with "?" for
  // the not-yet-entered rate.
  fireEvent.change(dialog.getByLabelText("Currency"), { target: { value: "SGD" } });
  await waitFor(() =>
    expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = ? IDR"),
  );

  // Typing the rate fills it into the equation, grouped as the field shows it.
  fireEvent.change(dialog.getByLabelText("Rate"), { target: { value: "12000" } });
  expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = 12,000 IDR");
});

it("the dialog hint and the Add rate button read the same figure, phone comma included", async () => {
  setViewport(1280);
  renderCard();
  await screen.findByTestId("fx-rate-table");
  const dialog = await openNewRate();
  await userEvent.click(dialog.getByLabelText("Month"));
  await userEvent.click(await screen.findByTestId("month-picker-cell-2026-02"));
  fireEvent.change(dialog.getByLabelText("Currency"), { target: { value: "SGD" } });
  const rate = dialog.getByLabelText("Rate");
  const add = dialog.getByRole("button", { name: "Add rate" });

  // A letter never reaches the field, so the hint can't show what Add refuses.
  await userEvent.type(rate, "x");
  expect(rate).toHaveValue("");
  await waitFor(() =>
    expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = ? IDR"),
  );
  // The empty rate is refused on click, with the reason beside it (#824).
  expect(add).toBeEnabled();
  await userEvent.click(add);
  expect(dialog.getByTestId("field-error-fx-rate")).toBeInTheDocument();

  await userEvent.type(rate, "12000,5");
  expect(rate).toHaveValue("12,000.5");
  expect(screen.getByTestId("fx-rate-hint")).toHaveTextContent("1 SGD = 12,000.5 IDR");
  expect(add).toBeEnabled();
});

it("desktop: Edit opens the dialog with month and currency locked and Save PATCHes the rate", async () => {
  setViewport(1280);
  let patched: { id: string; body: unknown } | null = null;
  server.use(
    http.patch("/api/fx-rates/:id", async ({ params, request }) => {
      patched = { id: String(params.id), body: await request.json() };
      return HttpResponse.json({ ...rates[0], rate: "16400" });
    }),
  );
  renderCard();

  await screen.findByTestId("fx-rate-table");
  pickRowAction(within(screen.getByTestId("fx-rate-row")), "Edit");

  const dialog = within(await screen.findByRole("dialog"));
  expect(dialog.getByLabelText("Month")).toBeDisabled();
  expect(dialog.getByLabelText("Currency")).toBeDisabled();
  const input = dialog.getByLabelText("Rate");
  expect(input).toHaveValue("16,250");
  fireEvent.change(input, { target: { value: "16400" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));

  await waitFor(() => expect(patched).not.toBeNull());
  expect(patched).toEqual({ id: "fx-1", body: { rate: "16400" } });
});

it("desktop: Save changes is blocked for a non-positive rate", async () => {
  setViewport(1280);
  renderCard();

  await screen.findByTestId("fx-rate-table");
  pickRowAction(within(screen.getByTestId("fx-rate-row")), "Edit");
  const dialog = within(await screen.findByRole("dialog"));
  fireEvent.change(dialog.getByLabelText("Rate"), { target: { value: "0" } });

  // A rate must be > 0 (ADR-0002): Save stays clickable and says why (#824).
  const save = dialog.getByRole("button", { name: "Save changes" });
  expect(save).toBeEnabled();
  fireEvent.click(save);
  expect(dialog.getByTestId("field-error-fx-rate")).toHaveTextContent(
    "Rate must be greater than zero.",
  );
});

it("desktop: paginates at 12 rows per page", async () => {
  setViewport(1280);
  // 13 monthly rates → two pages (12 + 1).
  const many: FxRate[] = Array.from({ length: 13 }, (_, i) => ({
    ...rates[0],
    id: `fx-${i}`,
    year_month: `2025-${String((i % 12) + 1).padStart(2, "0")}`,
  }));
  server.use(
    http.get("/api/fx-rates", () => HttpResponse.json(many)),
    http.get("/api/me", () => HttpResponse.json(me)),
  );
  renderWithProviders(<FxRatesCard />);

  await screen.findByTestId("fx-rate-table");
  expect(screen.getAllByTestId("fx-rate-row")).toHaveLength(12);

  fireEvent.click(screen.getByText("2"));
  expect(screen.getAllByTestId("fx-rate-row")).toHaveLength(1);
});

it("shows no pagination control for a single page", async () => {
  setViewport(1280);
  renderCard();
  await screen.findByTestId("fx-rate-table");
  expect(screen.queryByTestId("pagination-next")).not.toBeInTheDocument();
});

it("mobile: Edit from the card menu opens the dialog and Save PATCHes", async () => {
  setViewport(500);
  let patched: { id: string; body: unknown } | null = null;
  server.use(
    http.patch("/api/fx-rates/:id", async ({ params, request }) => {
      patched = { id: String(params.id), body: await request.json() };
      return HttpResponse.json({ ...rates[0], rate: "16400" });
    }),
  );
  renderCard();

  await screen.findByTestId("fx-rate-cards");
  pickRowAction(within(screen.getByTestId("fx-rate-row")), "Edit");

  const dialog = within(await screen.findByRole("dialog"));
  fireEvent.change(dialog.getByLabelText("Rate"), { target: { value: "16400" } });
  fireEvent.click(dialog.getByRole("button", { name: "Save changes" }));

  await waitFor(() => expect(patched).not.toBeNull());
  expect(patched).toEqual({ id: "fx-1", body: { rate: "16400" } });
});

// covers: INV-PRESENTATION-17
it("desktop: trash deletes at once with no dialog, raising one Undo toast", async () => {
  setViewport(1280);
  let deletes = 0;
  let undos = 0;
  server.use(
    http.delete("/api/fx-rates/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
    http.post("/api/fx-rates/:id/undo-delete", () => {
      undos += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderCard();
  await screen.findByTestId("fx-rate-table");

  pickRowAction(within(screen.getByTestId("fx-rate-row")), "Delete");
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
    http.delete("/api/fx-rates/:id", () => {
      deletes += 1;
      return new HttpResponse(null, { status: 204 });
    }),
  );
  renderCard();
  await screen.findByTestId("fx-rate-cards");

  pickRowAction(within(screen.getByTestId("fx-rate-row")), "Delete");
  await waitFor(() => expect(deletes).toBe(1));
  expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  expect(raisedToasts(mockedToast)).toHaveLength(1);
});
