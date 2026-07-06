import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { SortableHeader } from "@/components/SortableHeader";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { cn } from "@/lib/utils";
import type { PositionListRendererProps } from "@/components/positionList/types";

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
        <Table>
          <TableHeader>
            <TableRow>
              {columns.map((col) =>
                col.sortKey ? (
                  <SortableHeader
                    key={col.id}
                    label={col.label}
                    testId={`sort-${col.sortKey}`}
                    align={col.align}
                    active={sortKey === col.sortKey}
                    dir={sortDir}
                    onSort={() => onToggleSort(col.sortKey!)}
                  />
                ) : (
                  <TableHead key={col.id} className={cn(col.align === "right" && "text-right")}>
                    {col.label}
                  </TableHead>
                ),
              )}
              <TableHead className="w-12" />
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
                    className={cn(col.align === "right" && "text-right tabular-nums")}
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
