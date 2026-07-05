import { Card, CardContent } from "@/components/ui/card";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { cn } from "@/lib/utils";
import type { PositionListRendererProps } from "@/components/positionList/types";

// The mobile renderer: compact cards, essentials only. The card's content *is*
// the Position shared surface (uniform across every type), so it needs almost
// no per-type config — the title column heads the card, every other
// `mobileVisible` column stacks beneath it, and group extras are hidden unless
// they opted in with `mobile: "secondary"` (ADR-0043). No sort UI on the card;
// the core still hands rows down pre-sorted.
export function PositionListCards<T>({
  rows,
  columns,
  onSelect,
  onEdit,
  onDelete,
  actionsLabel,
  testIdPrefix,
}: PositionListRendererProps<T>) {
  const titleCol = columns.find((c) => c.isTitle);
  const bodyCols = columns.filter((c) => !c.isTitle && c.mobileVisible);
  return (
    <div className="space-y-3">
      {rows.map((row) => (
        <Card
          key={row.id}
          data-testid={`${testIdPrefix}-card`}
          className={cn(
            "cursor-pointer",
            row.terminated && "text-muted-foreground",
          )}
          onClick={() => onSelect(row.id)}
        >
          <CardContent className="flex items-start justify-between gap-3 p-4">
            <div className="min-w-0 space-y-1">
              {titleCol?.cell(row)}
              <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                {bodyCols.map((col) => (
                  <div key={col.id}>{col.cell(row)}</div>
                ))}
              </div>
            </div>
            <div onClick={(e) => e.stopPropagation()}>
              <RowActionsMenu
                label={actionsLabel}
                onEdit={() => onEdit(row.item)}
                onDelete={() => onDelete(row.item)}
              />
            </div>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
