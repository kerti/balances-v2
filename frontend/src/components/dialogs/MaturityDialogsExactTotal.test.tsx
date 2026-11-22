// The Maturity dialogs' "Total at maturity" adds principal and interest exactly
// (#991). formatCurrency itself goes through Number (ADR-0056), so the two-place
// display of 0.1 + 0.2 reads the same either way; the test pins the string handed
// to it, which is what a Number() sum would corrupt.
import { beforeEach, describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { UseMutationResult } from "@tanstack/react-query";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateMaturityTransactionDialog } from "@/components/dialogs/CreateMaturityTransactionDialog";
import { EditMaturityTransactionDialog } from "@/components/dialogs/EditMaturityTransactionDialog";
import type { UpdateTransactionMutationVariables } from "@/components/dialogs/EditTradeTransactionDialog";
import type { CreateInvestmentTransactionPayload } from "@/hooks/useInvestmentTransactions";
import type { InvestmentTransaction } from "@/api/types";

const formatCurrency = vi.hoisted(() => vi.fn());
vi.mock("@/lib/format", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/format")>();
  formatCurrency.mockImplementation(actual.formatCurrency);
  return { ...actual, formatCurrency };
});

function stub<V>() {
  return {
    mutate: vi.fn(),
    reset: vi.fn(),
    isError: false,
    isPending: false,
    error: null,
  } as unknown as UseMutationResult<unknown, unknown, V>;
}

function lastTotalArg(): string {
  const calls = formatCurrency.mock.calls.filter((c) => c[1] === "USD");
  return calls[calls.length - 1][0];
}

beforeEach(() => formatCurrency.mockClear());

// covers: INV-FINANCE-42
describe("Maturity dialogs total principal + interest exactly (#991)", () => {
  it("Create: 0.1 + 0.2 shows 0.3", async () => {
    renderWithProviders(
      <CreateMaturityTransactionDialog
        currency="USD"
        mutation={stub<CreateInvestmentTransactionPayload>()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /maturity/i }));
    await userEvent.type(screen.getByLabelText(/Principal/), "0.1");
    await userEvent.type(screen.getByLabelText(/Interest/), "0.2");
    expect(lastTotalArg()).toBe("0.3");
    expect(screen.getByText(/0\.30/)).toBeInTheDocument();
  });

  it("Create: a 16-digit principal keeps every digit", async () => {
    renderWithProviders(
      <CreateMaturityTransactionDialog
        currency="USD"
        mutation={stub<CreateInvestmentTransactionPayload>()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /maturity/i }));
    await userEvent.type(screen.getByLabelText(/Principal/), "9007199254740993");
    await userEvent.type(screen.getByLabelText(/Interest/), "2");
    expect(lastTotalArg()).toBe("9007199254740995");
  });

  it("Edit: 0.1 + 0.2 shows 0.3", () => {
    const transaction = {
      id: "t1",
      transaction_type: "maturity",
      transaction_date: "2026-03-15T00:00:00Z",
      currency: "USD",
      description: null,
      principal_amount: "0.1",
      interest_amount: "0.2",
      principal_disposition: "cash_out",
      interest_disposition: "cash_out",
    } as InvestmentTransaction;
    renderWithProviders(
      <EditMaturityTransactionDialog
        open
        onOpenChange={() => {}}
        transaction={transaction}
        mutation={stub<UpdateTransactionMutationVariables>()}
      />,
    );
    expect(lastTotalArg()).toBe("0.3");
    expect(screen.getByText(/0\.30/)).toBeInTheDocument();
  });
});
