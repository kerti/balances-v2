import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toastSaved, toastTradeAdded } from "@/lib/writeToast";
import { holdingMonthLabel } from "@/lib/recordSnapshot";
import { useRecordSnapshotRequest } from "@/lib/recordSnapshotRequest";
import i18n from "@/i18n";
import { useUndoableDelete } from "@/hooks/useUndoableDelete";
import type {
  InvestmentSnapshot,
  InvestmentTransaction,
  TransactionType,
  Disposition,
} from "@/api/types";

// Transactions live under /api/investments/{id}/transactions. One shared
// table per migration 00010; the repo's validateInvestmentTransactionType
// enforces the subtype→type matrix (Stock → Buy/Sell/Dividend/Fee; Bond →
// + Coupon, Maturity; TimeDeposit → Maturity only; etc.). Frontend dialogs
// per shape fork (Trade / CashIncome / Fee / Maturity) supply the right
// fields and leave the rest null.
//
// Mutations only invalidate the transactions list — unlike snapshots, the
// parent subtype list does not denormalize transactions. If a future
// "transaction count" or "last txn date" column lands on list rows, take
// the useInvestmentSnapshots listKey pattern.
//
// Exception: a Maturity transaction flips the parent investment to 'matured'
// server-side (ADR-0009 hard guard) AND upserts a truthful close snapshot at
// the maturity month (issue #25). Bond/TimeDeposit detail pages pass their
// subtype `detailKey` so create also invalidates:
//   [detailKey, id]                  — status badge + re-gates the (now-hidden)
//                                      transaction-create row
//   ['investment-snapshots', id]     — the close snapshot appears in the list
//                                      instantly (issue #56)
//   [detailKey]                      — the subtype collection inlines the
//                                      latest snapshot per row

export type CreateInvestmentTransactionPayload = {
  transaction_type: TransactionType;
  transaction_date: string;
  currency: string;
  description: string | null;
  amount: string | null;
  quantity: string | null;
  price_per_unit: string | null;
  principal_amount: string | null;
  interest_amount: string | null;
  principal_disposition: Disposition | null;
  interest_disposition: Disposition | null;
};

// transaction_type is immutable after creation — changing it would invalidate
// the shape. The Update payload mirrors Create minus transaction_type.
export type UpdateInvestmentTransactionPayload = Omit<
  CreateInvestmentTransactionPayload,
  "transaction_type"
>;

export function useInvestmentTransactions(investmentId: string | null) {
  return useQuery({
    queryKey: ["investment-transactions", investmentId],
    queryFn: () => api<InvestmentTransaction[]>(`/api/investments/${investmentId}/transactions`),
    enabled: !!investmentId,
  });
}

export function useCreateInvestmentTransaction(investmentId: string, detailKey?: string) {
  const qc = useQueryClient();
  const holding = useRecordSnapshotRequest();
  return useMutation({
    mutationFn: (payload: CreateInvestmentTransactionPayload) =>
      api<InvestmentTransaction>(`/api/investments/${investmentId}/transactions`, {
        method: "POST",
        body: JSON.stringify(payload),
      }),
    onSuccess: async (_created, payload) => {
      const refreshed = [
        qc.invalidateQueries({
          queryKey: ["investment-transactions", investmentId],
        }),
      ];
      // Maturity may have flipped the parent to 'matured' and upserted a close
      // snapshot — refresh the detail, the snapshot list, and the subtype
      // collection (issues #25 + #56).
      if (detailKey) {
        qc.invalidateQueries({ queryKey: [detailKey, investmentId] });
        qc.invalidateQueries({
          queryKey: ["investment-snapshots", investmentId],
        });
        qc.invalidateQueries({ queryKey: [detailKey] });
      }
      // A Buy or Sell on a position page offers "Update {Mon} holding →" (#830).
      // The shortcut reads the ledger total when it opens, so wait for the
      // refetch that includes this trade before showing it; the add itself has
      // already been written and the toast is still the one toast for the write.
      if (
        holding.available &&
        (payload.transaction_type === "buy" || payload.transaction_type === "sell")
      ) {
        await Promise.all(refreshed);
        const snapshots = qc.getQueryData<InvestmentSnapshot[]>([
          "investment-snapshots",
          investmentId,
        ]);
        toastTradeAdded(holdingMonthLabel(snapshots), holding.request);
        return;
      }
      toastSaved("added");
    },
  });
}

export function useUpdateInvestmentTransaction(investmentId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (args: { transactionId: string; payload: UpdateInvestmentTransactionPayload }) =>
      api<InvestmentTransaction>(
        `/api/investments/${investmentId}/transactions/${args.transactionId}`,
        {
          method: "PATCH",
          body: JSON.stringify(args.payload),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({
        queryKey: ["investment-transactions", investmentId],
      });
      toastSaved();
    },
  });
}

export function useDeleteInvestmentTransaction(investmentId: string) {
  return useUndoableDelete({
    path: (transactionId) => `/api/investments/${investmentId}/transactions/${transactionId}`,
    message: () => i18n.t("common:toast.deleted.transaction"),
    invalidate: (qc) => {
      qc.invalidateQueries({ queryKey: ["investment-transactions", investmentId] });
    },
  });
}
