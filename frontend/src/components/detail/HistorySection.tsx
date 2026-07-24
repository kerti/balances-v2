import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableHeader } from "@/components/ui/table";
import { PaginationControls } from "@/components/common/PaginationControls";
import type { HistorySectionSpec } from "@/components/detail/types";

// One history table — snapshots, or (for investments) a transaction ledger
// (ADR-0051). Owns the card + table + pagination + mobile table→cards reflow
// (Phase B) and never inspects columns: `header` is a neutral `<TableRow>` of
// `<TableHead>`s and `renderRow` returns a neutral `<TableRow>`. Holds its own
// page state so sibling sections paginate independently; the clamp derives
// during render so a delete that shrinks the range drops to the last existing
// page without an effect-driven setState (the M3.7 "stay on page" rule).
// `banner` (e.g. a reconcile warning) and `toolbar` (e.g. a transaction search
// box) are optional neutral nodes rendered above the table but never inspected —
// the descriptor owns their state, keeping this primitive presentation-neutral.
export function HistorySection<TRow>({
  testId,
  title,
  description,
  headerActions,
  toolbar,
  banner,
  emptyText,
  header,
  rows,
  renderRow,
  pageSize,
}: HistorySectionSpec<TRow>) {
  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const effectivePage = Math.min(page, totalPages);
  const pageRows = rows.slice((effectivePage - 1) * pageSize, effectivePage * pageSize);

  return (
    <Card data-testid={testId}>
      <CardHeader>
        <div className="flex items-center justify-between gap-4">
          <div>
            <CardTitle>{title}</CardTitle>
            {description && <CardDescription>{description}</CardDescription>}
          </div>
          {headerActions && <div className="flex flex-wrap gap-2">{headerActions}</div>}
        </div>
      </CardHeader>
      <CardContent className="p-0">
        {banner}
        {toolbar}
        {rows.length === 0 ? (
          <p className="p-6 text-sm text-muted-foreground">{emptyText}</p>
        ) : (
          <>
            <Table>
              <TableHeader>{header}</TableHeader>
              <TableBody>{pageRows.map((row, i) => renderRow(row, i))}</TableBody>
            </Table>
            {totalPages > 1 && (
              <div className="px-6 py-3 border-t">
                <PaginationControls
                  page={effectivePage}
                  totalPages={totalPages}
                  onPageChange={setPage}
                />
              </div>
            )}
          </>
        )}
      </CardContent>
    </Card>
  );
}
