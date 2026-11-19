// #934: Create forms pre-select nothing for "where did this come from?" — a
// default silently misstates the first month's living-expenses estimate in one
// common case — so submit is blocked until the household answers. Edit forms
// keep showing the stored value.
import { describe, expect, it } from "vitest";
import { http, HttpResponse, type JsonBodyType } from "msw";
import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { server } from "@/test/server";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateBankAccountDialog } from "@/components/dialogs/CreateBankAccountDialog";
import { CreateLiabilityDialog } from "@/components/dialogs/CreateLiabilityDialog";
import { CreateStockDialog } from "@/components/dialogs/CreateStockDialog";

const cases = [
  {
    name: "bank account (owned)",
    path: "/api/bank-accounts",
    dialog: () => <CreateBankAccountDialog />,
    fill: async () => {
      await userEvent.type(await screen.findByLabelText("Display name"), "Everyday");
      await userEvent.type(screen.getByLabelText("Bank name"), "Test Bank");
    },
    response: { asset: { id: "n1" }, details: {} },
    secondAnswer: /gift or inheritance/i,
  },
  {
    name: "stock (investment)",
    path: "/api/investments/stocks",
    dialog: () => <CreateStockDialog />,
    fill: async () => {
      await userEvent.type(await screen.findByLabelText("Display name"), "Test Co");
      await userEvent.type(screen.getByLabelText("Ticker"), "TEST");
      await userEvent.type(screen.getByLabelText("Exchange"), "XTST");
      await userEvent.selectOptions(screen.getByLabelText("Risk profile"), "medium");
    },
    response: { investment: { id: "n1" }, details: {} },
    secondAnswer: /gift or inheritance/i,
  },
  {
    name: "liability (owed)",
    path: "/api/liabilities",
    dialog: () => <CreateLiabilityDialog />,
    fill: async () => {
      await userEvent.type(await screen.findByLabelText("Display name"), "Car loan");
      await userEvent.type(screen.getByLabelText("Counterparty"), "Test Lender");
    },
    response: { id: "n1", details: {} },
    secondAnswer: /already owed it/i,
  },
];

function stubWrite(path: string, response: JsonBodyType) {
  const box: { body: Record<string, unknown> | null; count: number } = { body: null, count: 0 };
  server.use(
    http.get("/api/me", () =>
      HttpResponse.json({
        id: "u1",
        household_id: "h1",
        household_display_name: "Test Household",
        display_name: "Pat Owner",
        nickname: null,
        email: "pat@example.test",
        picture_url: null,
        locale: "en-GB",
        theme: "system",
        time_zone: "UTC",
        reporting_currency: "IDR",
        multi_currency_enabled: false,
        assumed_annual_inflation: "3.5",
        is_founder: true,
      }),
    ),
    http.post(path, async ({ request }) => {
      box.body = (await request.json()) as Record<string, unknown>;
      box.count += 1;
      return HttpResponse.json(response, { status: 201 });
    }),
  );
  return box;
}

// covers: INV-FINANCE-41
describe("Create dialogs make the household answer the entry-type question", () => {
  it.each(cases)("$name: starts with no radio checked", async (c) => {
    renderWithProviders(c.dialog());
    await userEvent.click(screen.getByRole("button"));
    const group = await screen.findByTestId("entry-type-options");
    for (const radio of within(group).getAllByRole("radio")) expect(radio).not.toBeChecked();
  });

  it.each(cases)("$name: refuses to submit unanswered and says why", async (c) => {
    const box = stubWrite(c.path, c.response);
    renderWithProviders(c.dialog());
    await userEvent.click(screen.getByRole("button"));
    await c.fill();
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    expect(await screen.findByText("Pick one so the first month adds up.")).toBeInTheDocument();
    const group = screen.getByTestId("entry-type-options");
    expect(within(group).getAllByRole("radio")[0]).toHaveAttribute("aria-invalid", "true");
    expect(box.count).toBe(0);
  });

  it.each(cases)("$name: sends the chosen value once answered", async (c) => {
    const box = stubWrite(c.path, c.response);
    renderWithProviders(c.dialog());
    await userEvent.click(screen.getByRole("button"));
    await c.fill();
    await userEvent.click(screen.getByRole("radio", { name: c.secondAnswer }));
    await userEvent.click(screen.getByRole("button", { name: "Create" }));

    await waitFor(() => expect(box.count).toBe(1));
    expect(box.body?.entry_type).toBe("newly_tracked");
  });
});
