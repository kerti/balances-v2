import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHeader } from "@/components/common/SortableHeader";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { cn } from "@/lib/utils";
import type { ColumnRole, PositionListRendererProps } from "@/components/positionList/types";

// One width per column role, shared by every list so Ownership, Status, Value
// and the actions land at the same x everywhere (#838). Name is the only
// flexible column: it absorbs leftover width, so hiding Status (#836) widens
// Name and never moves Value or the actions. Subtype columns share one width.
const ROLE_WIDTH: Record<ColumnRole, string> = {
  name: "",
  detail: "w-36",
  ownership: "w-32",
  status: "w-28",
  value: "w-44",
};
// Name, subtype and ownership text may wrap inside their own cell; the
// fixed-width roles stay on one line.
const WRAPS: ColumnRole[] = ["name", "detail", "ownership"];

// The web renderer: the dense, all-columns table. It iterates the core's
// resolved `columns` — sortable ones become a `SortableHeader`, the rest a
// plain `TableHead` — and wraps each column's neutral `cell` content in a
// `<TableCell>`. The trailing actions cell is fixed. No shared-surface markup
// lives here; it all comes down through `columns` (ADR-0043).
export function PositionListTable<T>({
  rows,
  columns,
  sortKey,
  sortDir,
  onToggleSort,
  onSelect,
  onEdit,
  onDelete,
  actionsLabel,
  testIdPrefix,
}: PositionListRendererProps<T>) {
  return (
    <Card>
      <CardContent className="p-0">
        <Table inset className="min-w-[48rem] table-fixed">
          <TableHeader>
            <TableRow>
              {columns.map((col) =>
                col.sortKey ? (
                  <SortableHeader
                    key={col.id}
                    label={col.label}
                    testId={`sort-${col.sortKey}`}
                    align={col.align}
                    className={ROLE_WIDTH[col.role]}
                    active={sortKey === col.sortKey}
                    dir={sortDir}
                    onSort={() => onToggleSort(col.sortKey!)}
                  />
                ) : (
                  <TableHead
                    key={col.id}
                    className={cn(ROLE_WIDTH[col.role], col.align === "right" && "text-right")}
                  >
                    {col.label}
                  </TableHead>
                ),
              )}
              <TableHead className="w-16" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.id}
                data-testid={`${testIdPrefix}-row`}
                className={cn("cursor-pointer", row.terminated && "text-muted-foreground")}
                onClick={() => onSelect(row.id)}
              >
                {columns.map((col) => (
                  <TableCell
                    key={col.id}
                    className={cn(
                      WRAPS.includes(col.role) && "break-words whitespace-normal",
                      col.align === "right" && "text-right",
                    )}
                  >
                    {col.cell(row)}
                  </TableCell>
                ))}
                <TableCell className="text-right">
                  <RowActionsMenu
                    label={actionsLabel}
                    onEdit={() => onEdit(row.item)}
                    onDelete={() => onDelete(row.item)}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
