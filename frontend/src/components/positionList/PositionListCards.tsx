import { Card, CardContent } from "@/components/ui/card";
import { CardField, CardFields } from "@/components/common/CardFields";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { cn } from "@/lib/utils";
import type { PositionListRendererProps } from "@/components/positionList/types";

// The mobile renderer: compact cards, essentials only. The card's content *is*
// the Position shared surface (uniform across every type), so it needs almost
// no per-type config — the title column heads the card, every other
// `mobileVisible` column stacks beneath it as a labelled key/value row (the
// card grammar of #849), and group extras are hidden unless they opted in with
// `mobile: "secondary"` (ADR-0043); Ownership is always shown. No sort UI on the card;
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
          className={cn("cursor-pointer py-0", row.terminated && "text-muted-foreground")}
          onClick={() => onSelect(row.id)}
        >
          <CardContent className="space-y-2 p-4">
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">{titleCol?.cell(row)}</div>
              <div onClick={(e) => e.stopPropagation()}>
                <RowActionsMenu
                  label={actionsLabel}
                  onEdit={() => onEdit(row.item)}
                  onDelete={() => onDelete(row.item)}
                  variant="outline"
                  triggerClassName="size-11 shrink-0"
                />
              </div>
            </div>
            <CardFields>
              {bodyCols.map((col) => (
                <CardField key={col.id} label={col.label}>
                  {col.cell(row)}
                </CardField>
              ))}
            </CardFields>
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
