import { useId, useState, type ReactNode } from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "@/lib/utils";

export type SnapshotBreakdownItem = { label: string; value: ReactNode };

type Props = {
  monthText: string;
  amountText: string;
  /** "Statement <date>" when the snapshot carries one. */
  statement?: string | null;
  /** With `statement`, the muted, single-line truncated second line; rows with
   *  neither stay one line. */
  note: string | null;
  /** Quantity × price / principal + accrued. Absent or empty: the row is plain
   *  and does not expand. */
  breakdown?: SnapshotBreakdownItem[];
  menu: ReactNode;
};

// The phone snapshot-history row (#952): `month · total value · ⋯`, a plain
// divided list rather than #849's labelled cards, on purpose — the repeated
// labels made a history of twelve months read as noise. Shared by the plain,
// quantity × price and accrued-interest renderers; the desktop table rows are
// untouched. Expandable rows put month + value in one real button
// (aria-expanded) and keep the ⋯ menu a sibling, so opening the menu never
// toggles the breakdown.
export function CompactSnapshotRow({
  monthText,
  amountText,
  statement,
  note,
  breakdown,
  menu,
}: Props) {
  const [open, setOpen] = useState(false);
  const panelId = useId();
  const expandable = !!breakdown && breakdown.length > 0;
  const subline = [statement, note].filter(Boolean).join(" · ");

  const summary = (
    <>
      <span className="flex min-h-11 items-center gap-3">
        <span className="min-w-0 flex-1 truncate font-medium">{monthText}</span>
        <span
          data-testid="snapshot-amount"
          className="shrink-0 text-right font-semibold tabular-nums"
        >
          {amountText}
        </span>
        {expandable && (
          <ChevronDown
            aria-hidden
            className={cn("size-4 shrink-0 text-muted-foreground transition-transform", {
              "rotate-180": open,
            })}
          />
        )}
      </span>
      {subline && (
        <span
          data-testid="snapshot-note"
          className="-mt-2 block truncate pb-2 text-xs text-muted-foreground"
        >
          {subline}
        </span>
      )}
    </>
  );

  return (
    <div data-testid="snapshot-row" className="border-b last:border-b-0">
      <div className="flex items-start gap-1">
        {expandable ? (
          <button
            type="button"
            aria-expanded={open}
            aria-controls={panelId}
            onClick={() => setOpen((o) => !o)}
            className="min-w-0 flex-1 rounded-sm text-left text-sm"
          >
            {summary}
          </button>
        ) : (
          <div className="min-w-0 flex-1 text-sm">{summary}</div>
        )}
        {menu}
      </div>
      {expandable && open && (
        <dl id={panelId} data-testid="snapshot-breakdown" className="space-y-1 pb-3 text-sm">
          {breakdown.map((item) => (
            <div key={item.label} className="flex justify-between gap-3">
              <dt className="shrink-0 text-muted-foreground">{item.label}</dt>
              <dd className="min-w-0 text-right tabular-nums">{item.value}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}
