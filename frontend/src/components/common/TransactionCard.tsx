import type { UseMutationResult } from "@tanstack/react-query";
import { Card, CardContent } from "@/components/ui/card";
import { TransactionRowMenu } from "@/components/common/TransactionRowMenu";
import { useTransactionRow } from "@/components/common/useTransactionRow";
import type { InvestmentTransaction } from "@/api/types";
import type { UpdateTransactionMutationVariables } from "@/components/dialogs/EditTradeTransactionDialog";

type Props<TUpdate, TDelete> = {
  transaction: InvestmentTransaction;
  quantityUnit: string;
  updateMutation: UseMutationResult<TUpdate, unknown, UpdateTransactionMutationVariables>;
  deleteMutation: UseMutationResult<TDelete, unknown, string>;
};

// Mobile leaf renderer for an investment transaction (ADR-0051 Phase B, "wide
// table → stacked cards"): the cash impact is promoted to the card headline so
// the figure the household member scans the ledger for reads with no horizontal
// scroll (signed and colour-coded, same convention as the desktop column), with
// the type + its quantity/price (or maturity split) detail, the date, and the
// note on their own lines below. Shares state, dialogs and the ⋮ menu with the
// desktop `TransactionRow` via `useTransactionRow`, and the same
// `transaction-row` / `transaction-amount` testids, so the two renderers stay in
// lockstep. The ⋮ trigger sizes to the 44px tap floor (INV-PRESENTATION-08).
export function TransactionCard<TUpdate, TDelete>({
  transaction,
  quantityUnit,
  updateMutation,
  deleteMutation,
}: Props<TUpdate, TDelete>) {
  const {
    dateText,
    label,
    detail,
    impactText,
    impactColorClass,
    description,
    onEdit,
    onDelete,
    dialogs,
  } = useTransactionRow(transaction, quantityUnit, updateMutation, deleteMutation);

  return (
    <>
      <Card data-testid="transaction-row">
        <CardContent className="space-y-2 p-4">
          <div className="flex items-start gap-2">
            <div className="flex-1 space-y-0.5">
              <div
                className={`text-lg font-semibold tabular-nums ${impactColorClass}`}
                data-testid="transaction-amount"
              >
                {impactText}
              </div>
              <div className="text-sm font-medium">{label}</div>
              {detail && <div className="text-sm text-muted-foreground">{detail}</div>}
              <div className="text-xs text-muted-foreground">{dateText}</div>
            </div>
            <TransactionRowMenu
              onEdit={onEdit}
              onDelete={onDelete}
              variant="outline"
              triggerClassName="size-11 shrink-0"
            />
          </div>
          {description && <p className="text-sm">{description}</p>}
        </CardContent>
      </Card>

      {dialogs}
    </>
  );
}
