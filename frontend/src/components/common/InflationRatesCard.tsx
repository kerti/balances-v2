import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PaginationControls } from "@/components/common/PaginationControls";
import { RowActionsMenu } from "@/components/positionList/RowActionsMenu";
import { InflationRateDialog } from "@/components/dialogs/InflationRateDialog";
import { useInflationRates, useDeleteInflationRate } from "@/hooks/useInflationRates";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatPercent, formatYearMonth } from "@/lib/format";
import type { InflationRate } from "@/api/types";

// One year of monthly figures per page — matches the Income screen's page size.
const PAGE_SIZE = 12;

// InflationRatesCard is the Settings ▸ Inflation Rates subpage's sole content:
// the manual monthly table (ADR-0048's "FX-like store"). The assumed-annual
// fallback lives on the Settings home page (Household section) instead — it's
// a single household preference, not a row-based lookup table like this one.
// Rates are annualized (YoY) percentages and may be negative (deflation). No
// card-level title/description — the page header (SettingsInflationRatesScreen)
// owns that.
//
// Every create and edit happens in `InflationRateDialog` (#840): the card
// toolbar holds the "New inflation rate" button and each row's ⋯ menu opens the
// same dialog in edit mode (only the rate is editable — the month is the row's
// identity).
//
// Mobile–web layout divergence (ADR-0050 B3b, #511): the entered rows split at
// the renderer — `useIsMobile` (768px) mounts stacked cards on phones and the
// wide table on desktop, so only one tree is ever in the DOM. Both leaves are
// fed the same rows and share the `inflation-rate-row` testid.
export function InflationRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useInflationRates();
  const deleteRate = useDeleteInflationRate();
  const isMobile = useIsMobile();

  const [editing, setEditing] = useState<InflationRate | null>(null);

  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil((rates?.length ?? 0) / PAGE_SIZE));
  const effectivePage = Math.min(page, totalPages);
  const pageRates = (rates ?? []).slice((effectivePage - 1) * PAGE_SIZE, effectivePage * PAGE_SIZE);

  const menu = (r: InflationRate, mobile: boolean) => (
    <RowActionsMenu
      label={t("inflation.rowActions")}
      onEdit={() => setEditing(r)}
      onDelete={() => deleteRate.mutate(r.id)}
      variant={mobile ? "outline" : "ghost"}
      triggerClassName={mobile ? "size-11" : undefined}
    />
  );

  return (
    <Card data-testid="inflation-rates-card">
      <CardContent className="space-y-4">
        <div className="flex justify-end">
          <InflationRateDialog mode="create" />
        </div>

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("inflation.empty")}</p>
        )}

        {rates &&
          rates.length > 0 &&
          (isMobile ? (
            <div className="space-y-2" data-testid="inflation-rate-cards">
              {pageRates.map((r) => (
                <InflationRateCard key={r.id} rate={r} menu={menu(r, true)} />
              ))}
            </div>
          ) : (
            <Table data-testid="inflation-rate-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("inflation.month")}</TableHead>
                  <TableHead className="text-right">{t("inflation.rate")}</TableHead>
                  <TableHead className="w-12"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageRates.map((r) => (
                  <TableRow key={r.id} data-testid="inflation-rate-row">
                    <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                    <TableCell className="text-right" data-testid="inflation-rate-value">
                      {formatPercent(r.rate)}
                    </TableCell>
                    <TableCell className="text-right">{menu(r, false)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ))}

        {totalPages > 1 && (
          <PaginationControls page={effectivePage} totalPages={totalPages} onPageChange={setPage} />
        )}
      </CardContent>
      {editing && (
        <InflationRateDialog
          key={editing.id}
          mode="edit"
          rate={editing}
          open
          onOpenChange={(open) => {
            if (!open) setEditing(null);
          }}
        />
      )}
    </Card>
  );
}

// Mobile leaf (ADR-0050 "wide table → stacked cards"): one card per inflation
// row. The rate — the annualized figure the user came to check — is promoted to
// the headline (a trailing "%" makes the unit explicit at a glance) with the
// month below. The ⋯ menu trigger is sized to the 44px tap floor
// (INV-PRESENTATION-08).
function InflationRateCard({ rate, menu }: { rate: InflationRate; menu: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3" data-testid="inflation-rate-row">
      <div className="min-w-0 flex-1">
        <div className="text-lg font-semibold" data-testid="inflation-rate-value">
          {formatPercent(rate.rate)}
        </div>
        <div className="text-sm text-muted-foreground">{formatYearMonth(rate.year_month)}</div>
      </div>
      <div className="shrink-0">{menu}</div>
    </div>
  );
}
