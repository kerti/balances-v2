import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { PaginationControls } from "@/components/common/PaginationControls";
import { IncomeRowDesktop } from "@/components/income/IncomeRowDesktop";
import { IncomeCard } from "@/components/income/IncomeCard";
import { useIsMobile } from "@/hooks/use-mobile";
import type { Income } from "@/api/types";

type Props = {
  rows: Income[];
  page: number;
  totalPages: number;
  onPageChange: (page: number) => void;
};

// The Income list's mobile–web split point (ADR-0050). The IncomeScreen
// container owns the query, month/regularity filters, headline stats and
// pagination state, and hands one presentation-neutral page of rows here;
// `useIsMobile` (768px) then picks which renderer mounts — stacked cards on
// phones, the wide table on desktop — so only one tree is ever in the DOM. Both
// leaves are fed the same rows and share the `income-*` data-testids.
export function IncomeList({ rows, page, totalPages, onPageChange }: Props) {
  const { t } = useTranslation(["income"]);
  const isMobile = useIsMobile();

  const pagination = totalPages > 1 && (
    <PaginationControls page={page} totalPages={totalPages} onPageChange={onPageChange} />
  );

  if (isMobile) {
    return (
      <div className="space-y-3" data-testid="income-card-list">
        {rows.map((row) => (
          <IncomeCard key={row.id} income={row} />
        ))}
        {pagination}
      </div>
    );
  }

  return (
    <Card>
      <CardContent className="p-0">
        <Table data-testid="income-table">
          <TableHeader>
            <TableRow>
              <TableHead>{t("income:tableHeaders.date")}</TableHead>
              <TableHead>{t("income:tableHeaders.category")}</TableHead>
              <TableHead className="text-right">{t("income:tableHeaders.amount")}</TableHead>
              <TableHead>{t("income:tableHeaders.description")}</TableHead>
              <TableHead>{t("income:tableHeaders.ownership")}</TableHead>
              <TableHead className="w-12"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <IncomeRowDesktop key={row.id} income={row} />
            ))}
          </TableBody>
        </Table>
        {pagination && <div className="border-t px-6 py-3">{pagination}</div>}
      </CardContent>
    </Card>
  );
}
