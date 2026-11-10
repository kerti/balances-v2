// Inline validation in the real dialogs (#824, ADR-0058 §5): an empty New bank
// account lists every missing field, Record Buy and Add rate stay clickable when
// invalid, and SNAPSHOT_MONTH_EXISTS renders beside the month (and the picker
// fills its row, #865).
import { describe, expect, it, vi } from "vitest";
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import i18n from "@/i18n";
import { ApiError } from "@/api/client";
import { renderWithProviders } from "@/test/renderWithProviders";
import { CreateBankAccountDialog } from "@/components/dialogs/CreateBankAccountDialog";
import { CreateSnapshotDialog } from "@/components/dialogs/CreateSnapshotDialog";
import { CreateTradeTransactionDialog } from "@/components/dialogs/CreateTradeTransactionDialog";
import { FxRateDialog } from "@/components/dialogs/FxRateDialog";

vi.mock("@/hooks/useSession", () => ({
  useSession: () => ({ data: { id: "u1", reporting_currency: "IDR" } }),
}));
vi.mock("@/hooks/useBankAccounts", () => ({
  useCreateBankAccount: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null }),
}));
vi.mock("@/hooks/useFxRates", () => ({
  useCreateFxRate: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null }),
  useUpdateFxRate: () => ({ mutate: vi.fn(), reset: vi.fn(), isPending: false, error: null }),
}));

const required = i18n.t("common:validation.required");

function stub(error: unknown = null) {
  const mutate = vi.fn();
  return {
    mutate,
    mutation: {
      mutate,
      reset: vi.fn(),
      isError: error != null,
      isPending: false,
      error,
    } as unknown,
  };
}

// covers: INV-PRESENTATION-19
describe("inline validation", () => {
  it("an empty New bank account shows every missing field at once", async () => {
    renderWithProviders(<CreateBankAccountDialog />);
    await userEvent.click(screen.getByRole("button", { name: /bank account/i }));
    const create = await screen.findByRole("button", { name: i18n.t("common:actions.create") });
    expect(create).toBeEnabled();

    await userEvent.click(create);

    for (const id of ["display_name", "bank_name", "account_number"]) {
      expect(screen.getByTestId(`field-error-${id}`)).toHaveTextContent(required);
    }
    // Currency is pre-filled, so it is not flagged.
    expect(screen.queryByTestId("field-error-native_currency")).not.toBeInTheDocument();
  });

  it("Record Buy is clickable when empty and says which fields are missing", async () => {
    const { mutate, mutation } = stub();
    renderWithProviders(
      <CreateTradeTransactionDialog
        currency="IDR"
        txnType="buy"
        quantityUnit="sh"
        mutation={mutation as never}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Buy" }));
    const record = await screen.findByRole("button", {
      name: i18n.t("investments:trade.recordBuy"),
    });
    expect(record).toBeEnabled();

    await userEvent.click(record);

    expect(mutate).not.toHaveBeenCalled();
    expect(screen.getByTestId("field-error-trade_quantity")).toHaveTextContent(required);
    expect(screen.getByTestId("field-error-trade_price")).toHaveTextContent(required);
  });

  it("Add rate is clickable when empty and says which fields are missing", async () => {
    renderWithProviders(<FxRateDialog mode="create" />);
    await userEvent.click(screen.getByTestId("fx-new-rate"));
    const add = await screen.findByRole("button", { name: i18n.t("settings:fx.addRate") });
    expect(add).toBeEnabled();

    await userEvent.click(add);

    expect(screen.getByTestId("field-error-fx-month")).toHaveTextContent(required);
    expect(screen.getByTestId("field-error-fx-currency")).toHaveTextContent(required);
    expect(screen.getByTestId("field-error-fx-rate")).toHaveTextContent(required);
  });

  it("SNAPSHOT_MONTH_EXISTS renders beside the month field, not as a form message", async () => {
    const error = new ApiError(409, "conflict", { code: "SNAPSHOT_MONTH_EXISTS" });
    const { mutation } = stub(error);
    renderWithProviders(<CreateSnapshotDialog currency="IDR" mutation={mutation as never} />);
    await userEvent.click(screen.getByRole("button", { name: i18n.t("common:snapshot.trigger") }));

    expect(await screen.findByTestId("field-error-year_month")).toHaveTextContent(
      i18n.t("errors:code.SNAPSHOT_MONTH_EXISTS"),
    );
    expect(screen.queryByTestId("form-error")).not.toBeInTheDocument();
  });

  it("the month picker fills its row above the statement date (#865)", async () => {
    const { mutation } = stub();
    renderWithProviders(<CreateSnapshotDialog currency="IDR" mutation={mutation as never} />);
    await userEvent.click(screen.getByRole("button", { name: i18n.t("common:snapshot.trigger") }));

    const trigger = await screen.findByTestId("year_month");
    const group = trigger.parentElement as HTMLElement;
    expect(group).toHaveClass("w-full");
    // Month and statement date are separate rows, never a two-column grid.
    expect(group.parentElement).not.toHaveClass("grid-cols-2");
    expect(screen.getByLabelText(i18n.t("common:fields.statementDate"))).toBeInTheDocument();
  });
});
