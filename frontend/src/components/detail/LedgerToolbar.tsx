import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { formatCurrency } from "@/lib/format";
import { ledgerTotals } from "@/lib/ledgerTotals";
import type { InvestmentSubtype } from "@/lib/lifecycle";
import type { InvestmentTransaction } from "@/api/types";

// The strip above every investment transaction ledger (PAT-10, #830): a totals
// line summing the kinds this subtype records (lib/ledgerTotals) beside the
// search box. Rendered only once the ledger has a row - an empty ledger shows its
// own empty state and has nothing to total or search.
export function LedgerToolbar({
  subtype,
  transactions,
  currency,
  search,
  onSearchChange,
}: {
  subtype: InvestmentSubtype;
  transactions: InvestmentTransaction[];
  // The position's currency: what a total with no rows of its kind reads in.
  currency: string;
  search: string;
  onSearchChange: (value: string) => void;
}) {
  const { t } = useTranslation(["investments"]);
  if (transactions.length === 0) return null;
  const totals = ledgerTotals(subtype, transactions, currency);
  return (
    <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 border-b px-6 py-3">
      <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm" data-testid="ledger-totals">
        {totals.map(({ key, amounts }) => (
          <div key={key} data-testid={`ledger-total-${key}`}>
            <span className="text-muted-foreground">
              {t(`investments:transactions.totals.${key}`)}
            </span>{" "}
            <span>{amounts.map((a) => formatCurrency(a.amount, a.currency)).join(" · ")}</span>
          </div>
        ))}
      </div>
      <Input
        data-testid="txn-search"
        placeholder={t("investments:transactions.searchPlaceholder")}
        value={search}
        onChange={(e) => onSearchChange(e.target.value)}
        className="w-full md:max-w-xs"
      />
    </div>
  );
}
