// The Buy/Sell dialogs own the trade's amount (#991): it is quantity × price,
// formed with exact BigInt arithmetic and rounded once to the column's 4 places,
// never through Number, so what the preview shows and what is sent are the same
// string the server will store.
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import {
  EditTradeTransactionDialog,
  type UpdateTransactionMutationVariables,
} from "@/components/dialogs/EditTradeTransactionDialog";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";
import type { InvestmentTransaction } from "@/api/types";

function stub<V>() {
  const mutate = vi.fn();
  const mutation = {
    mutate,
    reset: vi.fn(),
    isError: false,
    isPending: false,
    error: null,
  } as unknown as UseMutationResult<unknown, unknown, V>;
  return { mutate, mutation };
}

async function createAndSubmit(quantity: string, price: string) {
  const { mutate, mutation } = stub<CreateInvestmentTransactionPayload>();
  renderWithProviders(
    <CreateTradeTransactionDialog
      currency="USD"
      txnType="buy"
      quantityUnit="sh"
      mutation={mutation}
    />,
  );
  await userEvent.click(screen.getByRole("button", { name: "Buy" }));
  await userEvent.type(screen.getByLabelText(/Quantity/), quantity);
  await userEvent.type(screen.getByLabelText(/Price per unit/), price);
  await userEvent.click(screen.getByRole("button", { name: "Record buy" }));
  return mutate;
}

async function editAndSubmit(quantity: string, price: string) {
  const { mutate, mutation } = stub<UpdateTransactionMutationVariables>();
  const transaction = {
    id: "t1",
    transaction_type: "sell",
    transaction_date: "2026-03-15T00:00:00Z",
    currency: "USD",
    description: null,
    quantity: "1",
    price_per_unit: "1",
  } as InvestmentTransaction;
  renderWithProviders(
    <EditTradeTransactionDialog
      open
      onOpenChange={() => {}}
      transaction={transaction}
      quantityUnit="sh"
      mutation={mutation}
    />,
  );
  const q = screen.getByLabelText(/Quantity/);
  const p = screen.getByLabelText(/Price per unit/);
  await userEvent.clear(q);
  await userEvent.type(q, quantity);
  await userEvent.clear(p);
  await userEvent.type(p, price);
  await userEvent.click(screen.getByRole("button", { name: "Save changes" }));
  return mutate;
}

const cases: Array<[string, string, string, string]> = [
  ["a float artefact", "0.1", "3", "0.3"],
  [
    "a product wider than a double holds",
    "123456789012",
    "1234567890.1234",
    "152415787531954592444.0808",
  ],
  ["a fifth decimal of 5 rounds up", "0.00005", "1", "0.0001"],
  ["a fifth decimal below 5 rounds down", "0.00004", "1", "0"],
  ["eight-place quantity, four-place price", "1.12345678", "2.1234", "2.3855"],
];

// covers: INV-FINANCE-42
describe("Trade dialogs send an exact amount (#991)", () => {
  describe.each(cases)("%s", (_name, quantity, price, amount) => {
    it(`Create: ${quantity} × ${price} sends ${amount}`, async () => {
      const mutate = await createAndSubmit(quantity, price);
      expect(mutate).toHaveBeenCalledTimes(1);
      expect(mutate.mock.calls[0][0]).toMatchObject({ amount });
    });

    it(`Edit: ${quantity} × ${price} sends ${amount}`, async () => {
      const mutate = await editAndSubmit(quantity, price);
      expect(mutate).toHaveBeenCalledTimes(1);
      expect(mutate.mock.calls[0][0].payload).toMatchObject({ amount });
    });
  });
});
