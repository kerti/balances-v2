// Free-text filter for the investment-transaction list on the five detail
// screens (Stock / MutualFund / Bond / TimeDeposit / Gold). Matches against
// the localised transaction-type label (so "buy" matches "Beli" too) plus
// the user-entered description. Investment transactions don't carry a
// separate counterparty field — description doubles as that surface.
import i18n from "@/i18n";
import type { InvestmentTransaction } from "@/api/types";

export function matchesTxnSearch(
  tx: InvestmentTransaction,
  query: string,
): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  const typeLabel = i18n
    .t(`investments:transactionType.${tx.transaction_type}`, {
      defaultValue: tx.transaction_type,
    })
    .toLowerCase();
  const desc = (tx.description ?? "").toLowerCase();
  return typeLabel.includes(q) || desc.includes(q);
}
