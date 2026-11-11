// Record Buy with a grouped price (audit BUG-01): before the shared
// NumericInput, `6,500` made Number() fail, so the cash-out read "—" and Save
// stayed disabled.
import { describe, expect, it, vi } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import i18n from "@/i18n";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";

function mutationStub() {
  const mutate = vi.fn();
  const mutation = {
    mutate,
    reset: vi.fn(),
    isError: false,
    isPending: false,
    error: null,
  } as unknown as UseMutationResult<unknown, unknown, CreateInvestmentTransactionPayload>;
  return { mutate, mutation };
}

async function openBuy() {
  const { mutate, mutation } = mutationStub();
  renderWithProviders(
    <CreateTradeTransactionDialog
      currency="IDR"
      txnType="buy"
      quantityUnit="sh"
      mutation={mutation}
    />,
  );
  await userEvent.click(screen.getByRole("button", { name: "Buy" }));
  return { mutate };
}

// covers: INV-PRESENTATION-13
describe("CreateTradeTransactionDialog (Buy)", () => {
  it("derives the cash-out from a pasted grouped price and sends canonical values", async () => {
    const { mutate } = await openBuy();
    await userEvent.type(screen.getByLabelText(/Quantity/), "100");
    await userEvent.click(screen.getByLabelText(/Price per unit/));
    await userEvent.paste("6,500");

    expect(screen.getByLabelText(/Price per unit/)).toHaveValue("6,500");
    // Intl puts a no-break space after the code; match on the figure.
    expect(screen.getByText(/650,000/)).toBeInTheDocument();
    const save = screen.getByRole("button", { name: /record|save/i });
    expect(save).toBeEnabled();

    await userEvent.click(save);
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ quantity: "100", price_per_unit: "6500", amount: "650000" }),
      expect.anything(),
    );
  });

  it("groups a typed price live", async () => {
    await openBuy();
    await userEvent.type(screen.getByLabelText(/Price per unit/), "6500");
    expect(screen.getByLabelText(/Price per unit/)).toHaveValue("6,500");
  });
});

// covers: INV-SNAPSHOTS-18
describe("CreateTradeTransactionDialog says the trade stands alone (#830)", () => {
  const EN =
    "This records the trade. Update the holding in this month's snapshot, and the bank balance in that account.";

  it("states that the holding and the bank balance are updated separately", async () => {
    await openBuy();
    expect(screen.getByTestId("trade-separate-note")).toHaveTextContent(EN);
  });

  it("says it on Sell too, and in Indonesian", async () => {
    const { mutation } = mutationStub();
    renderWithProviders(
      <CreateTradeTransactionDialog
        currency="IDR"
        txnType="sell"
        quantityUnit="sh"
        mutation={mutation}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Sell" }));
    expect(screen.getByTestId("trade-separate-note")).toHaveTextContent(EN);
    cleanup();

    await i18n.changeLanguage("id-ID");
    try {
      renderWithProviders(
        <CreateTradeTransactionDialog
          currency="IDR"
          txnType="buy"
          quantityUnit="sh"
          mutation={mutation}
        />,
      );
      await userEvent.click(screen.getByRole("button", { name: "Beli" }));
      expect(screen.getByTestId("trade-separate-note")).toHaveTextContent(
        "Ini mencatat transaksi. Perbarui kepemilikan di snapshot bulan ini, dan saldo di rekening banknya.",
      );
    } finally {
      await i18n.changeLanguage("en-GB");
    }
  });

  it("only submits the trade: the dialog hands the mutation one trade payload and nothing else", async () => {
    const { mutate } = await openBuy();
    await userEvent.type(screen.getByLabelText(/Quantity/), "10");
    await userEvent.type(screen.getByLabelText(/Price per unit/), "40");
    await userEvent.click(screen.getByRole("button", { name: "Record buy" }));
    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate).toHaveBeenCalledWith(
      expect.objectContaining({ transaction_type: "buy", quantity: "10", amount: "400" }),
      expect.anything(),
    );
  });
});
