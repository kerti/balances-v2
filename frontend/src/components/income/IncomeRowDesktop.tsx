import { TableCell, TableRow } from "@/components/ui/table";
import { IncomeRowMenu } from "@/components/income/IncomeRowMenu";
import { useIncomeRow } from "@/components/income/useIncomeRow";
import type { Income } from "@/api/types";

type Props = {
  income: Income;
};

// Desktop leaf renderer (ADR-0050): the wide table row. Fed the same projection
// and handlers as the mobile card via `useIncomeRow`; shares the `income-*`
// data-testids so one assertion holds across renderers.
export function IncomeRowDesktop({ income }: Props) {
  const row = useIncomeRow(income);
  const { RegularityIcon } = row;

  return (
    <>
      <TableRow data-testid="income-row">
        <TableCell className="whitespace-nowrap">{row.dateText}</TableCell>
        <TableCell>
          <div className="flex items-center gap-1.5">
            <span className="inline-flex items-center rounded-full border px-2 py-0.5 text-xs">
              {row.categoryChip}
            </span>
            <RegularityIcon
              className="size-3.5 text-muted-foreground"
              aria-label={row.regularityLabel}
              data-testid={`regularity-${row.regularity}`}
            />
          </div>
        </TableCell>
        <TableCell
          className="whitespace-nowrap text-right font-medium tabular-nums"
          data-testid="income-amount"
        >
          {row.amountText}
        </TableCell>
        <TableCell className="text-sm text-muted-foreground">
          {row.description ?? <span className="text-muted-foreground/60">{"—"}</span>}
        </TableCell>
        <TableCell className="text-xs text-muted-foreground">{row.ownerLabel}</TableCell>
        <TableCell className="text-right">
          <IncomeRowMenu
            onEdit={row.onEdit}
            onDuplicate={row.onDuplicate}
            onDelete={row.onDelete}
          />
        </TableCell>
      </TableRow>

      {row.dialogs}
    </>
  );
}
