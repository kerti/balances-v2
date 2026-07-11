import { useTranslation } from "react-i18next";
import { TableCell } from "@/components/ui/table";
import { formatDate } from "@/lib/format";

type Props = {
  count: number;
  lastDate: string | null;
};

// Shared "Activity" cell for investment subtype list rows (issue #67): the
// ledger's transaction count plus the most-recent transaction date. Every
// subtype list row reuses this so the column reads identically across screens.
// An empty ledger shows a muted dash.
export function TransactionActivityCell({ count, lastDate }: Props) {
  const { t } = useTranslation("common");

  if (count === 0) {
    return (
      <TableCell>
        <span className="text-muted-foreground">—</span>
      </TableCell>
    );
  }

  return (
    <TableCell>
      <div className="text-sm">{t("common:activity.count", { count })}</div>
      {lastDate && (
        <div className="text-xs text-muted-foreground">
          {t("common:activity.last", { date: formatDate(lastDate) })}
        </div>
      )}
    </TableCell>
  );
}
