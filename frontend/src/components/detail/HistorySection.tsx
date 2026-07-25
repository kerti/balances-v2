import { useState } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableHeader } from "@/components/ui/table";
import { PaginationControls } from "@/components/common/PaginationControls";
import { useIsMobile } from "@/hooks/use-mobile";
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
  renderCard,
  pageSize,
}: HistorySectionSpec<TRow>) {
  const isMobile = useIsMobile();
  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil(rows.length / pageSize));
  const effectivePage = Math.min(page, totalPages);
  const pageRows = rows.slice((effectivePage - 1) * pageSize, effectivePage * pageSize);

  // Below 768px render a stacked card list once the shape supplies `renderCard`
  // (Phase B); otherwise the wide table, which is always the desktop renderer. A
  // shape not yet carded falls back to the table on mobile rather than rendering
  // nothing.
  const asCards = isMobile && !!renderCard;

  const pagination = totalPages > 1 && (
    <div className="border-t px-6 py-3">
      <PaginationControls page={effectivePage} totalPages={totalPages} onPageChange={setPage} />
    </div>
  );

  return (
    <Card data-testid={testId}>
      <CardHeader>
        <div className="flex flex-col items-start gap-3 md:flex-row md:items-center md:justify-between md:gap-4">
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
        ) : asCards ? (
          <>
            <div className="space-y-3 p-4" data-testid={`${testId}-cards`}>
              {pageRows.map((row, i) => renderCard!(row, i))}
            </div>
            {pagination}
          </>
        ) : (
          <>
            <Table data-testid={`${testId}-table`}>
              <TableHeader>{header}</TableHeader>
              <TableBody>{pageRows.map((row, i) => renderRow(row, i))}</TableBody>
            </Table>
            {pagination}
          </>
        )}
      </CardContent>
    </Card>
  );
}
