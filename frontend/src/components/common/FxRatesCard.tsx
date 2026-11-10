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
import { FxRateDialog } from "@/components/dialogs/FxRateDialog";
import { useFxRates, useDeleteFxRate } from "@/hooks/useFxRates";
import { useSession } from "@/hooks/useSession";
import { useIsMobile } from "@/hooks/use-mobile";
import { fxEquation, fxPair } from "@/lib/fxPair";
import { formatYearMonth, groupCanonical } from "@/lib/format";
import type { FxRate } from "@/api/types";

// One year of monthly rates per page — matches the Income screen's page size.
const PAGE_SIZE = 12;

// FxRatesCard is the Settings ▸ Exchange Rates subpage's sole content: the
// manual monthly FX table (ADR-0024). Only rendered when multi-currency is on
// (gated by the caller) — there is nothing to convert otherwise. No card-level
// title/description — the page header (SettingsFxRatesScreen) owns that.
//
// Every create and edit happens in `FxRateDialog` (#840): the card toolbar holds
// the "New exchange rate" button and each row's ⋯ menu opens the same dialog in
// edit mode (only the rate is editable — month + currency are the row's
// identity).
//
// Mobile–web layout divergence (ADR-0050 B3b, #511): the entered rows split at
// the renderer — `useIsMobile` (768px) mounts stacked cards on phones and the
// wide table on desktop, so only one tree is ever in the DOM. Both leaves are
// fed the same rows and share the `fx-rate-row` testid.
export function FxRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useFxRates();
  const { data: me } = useSession();
  const deleteRate = useDeleteFxRate();
  const isMobile = useIsMobile();

  const base = me?.reporting_currency?.toUpperCase();

  const [editing, setEditing] = useState<FxRate | null>(null);

  const [page, setPage] = useState(1);
  const totalPages = Math.max(1, Math.ceil((rates?.length ?? 0) / PAGE_SIZE));
  const effectivePage = Math.min(page, totalPages);
  const pageRates = (rates ?? []).slice((effectivePage - 1) * PAGE_SIZE, effectivePage * PAGE_SIZE);

  const menu = (r: FxRate, mobile: boolean) => (
    <RowActionsMenu
      label={t("fx.rowActions")}
      onEdit={() => setEditing(r)}
      onDelete={() => deleteRate.mutate(r.id)}
      variant={mobile ? "outline" : "ghost"}
      triggerClassName={mobile ? "size-11" : undefined}
    />
  );

  return (
    <Card data-testid="fx-rates-card">
      <CardContent className="space-y-4">
        <div className="flex justify-end">
          <FxRateDialog mode="create" />
        </div>

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("fx.empty")}</p>
        )}

        {rates &&
          rates.length > 0 &&
          (isMobile ? (
            <div className="space-y-2" data-testid="fx-rate-cards">
              {pageRates.map((r) => (
                <FxRateCard key={r.id} rate={r} base={base} menu={menu(r, true)} />
              ))}
            </div>
          ) : (
            <Table data-testid="fx-rate-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("fx.month")}</TableHead>
                  <TableHead>{t("fx.currency")}</TableHead>
                  <TableHead>{t("fx.rate")}</TableHead>
                  <TableHead className="w-12"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pageRates.map((r) => (
                  <TableRow key={r.id} data-testid="fx-rate-row">
                    <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                    <TableCell>{fxPair(r.currency, base)}</TableCell>
                    <TableCell data-testid="fx-rate-value">{groupCanonical(r.rate)}</TableCell>
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
        <FxRateDialog
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

// Mobile leaf (ADR-0050 "wide table → stacked cards"): one card per FX row. The
// rate is promoted to the headline as a full **equation** (`1 USD = 15600 IDR`)
// — a bare number beside "USD → IDR" would misread as "15600 USD"; the equation
// binds the value to the reporting currency it actually is. The month sits
// below. The ⋯ menu trigger is sized to the 44px tap floor (INV-PRESENTATION-08).
function FxRateCard({
  rate,
  base,
  menu,
}: {
  rate: FxRate;
  base: string | undefined;
  menu: React.ReactNode;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3" data-testid="fx-rate-row">
      <div className="min-w-0 flex-1">
        <div className="text-lg font-semibold" data-testid="fx-rate-value">
          {fxEquation(rate.currency, rate.rate, base)}
        </div>
        <div className="text-sm text-muted-foreground">{formatYearMonth(rate.year_month)}</div>
      </div>
      <div className="shrink-0">{menu}</div>
    </div>
  );
}
