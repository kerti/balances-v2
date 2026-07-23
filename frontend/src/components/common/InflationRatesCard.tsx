import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Trash2 } from "lucide-react";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { errorMessage } from "@/lib/errorMessage";
import {
  useInflationRates,
  useCreateInflationRate,
  useDeleteInflationRate,
} from "@/hooks/useInflationRates";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatYearMonth } from "@/lib/format";
import type { InflationRate } from "@/api/types";

// InflationRatesCard is the Settings ▸ Inflation Rates subpage's sole content:
// the manual monthly table (ADR-0048's "FX-like store"). The assumed-annual
// fallback lives on the Settings home page (Household section) instead — it's
// a single household preference, not a row-based lookup table like this one.
// Rates are annualized (YoY) percentages and may be negative (deflation). No
// card-level title/description — the page header (SettingsInflationRatesScreen)
// owns that.
//
// Mobile–web layout divergence (ADR-0050 B3b, #511): the add form is shared, but
// the entered rows split at the renderer — `useIsMobile` (768px) mounts stacked
// cards on phones and the wide table on desktop, so only one tree is ever in the
// DOM. Both leaves are fed the same rows and share the `inflation-rate-row` testid.
export function InflationRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useInflationRates();
  const createRate = useCreateInflationRate();
  const deleteRate = useDeleteInflationRate();
  const isMobile = useIsMobile();

  const [month, setMonth] = useState("");
  const [rate, setRate] = useState("");

  const add = () => {
    createRate.mutate(
      { year_month: month, rate },
      {
        onSuccess: () => {
          setMonth("");
          setRate("");
        },
      },
    );
  };

  // A rate may be negative (deflation) or zero; only require a parseable number.
  const canAdd = month !== "" && rate.trim() !== "" && !Number.isNaN(Number(rate));

  return (
    <Card data-testid="inflation-rates-card">
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="inflation-month">{t("inflation.month")}</Label>
            <Input
              id="inflation-month"
              type="month"
              className="w-40"
              value={month}
              onChange={(e) => setMonth(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="inflation-rate">{t("inflation.rate")}</Label>
            <Input
              id="inflation-rate"
              inputMode="decimal"
              className="w-36"
              // Example numeric rate; not translatable copy.
              placeholder={"3.5"}
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </div>
          <Button onClick={add} disabled={!canAdd || createRate.isPending}>
            {t("inflation.addRate")}
          </Button>
        </div>

        {createRate.isError && (
          <p className="text-sm text-destructive">{errorMessage(createRate.error)}</p>
        )}

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("inflation.empty")}</p>
        )}

        {rates &&
          rates.length > 0 &&
          (isMobile ? (
            <div className="space-y-2" data-testid="inflation-rate-cards">
              {rates.map((r) => (
                <InflationRateCard
                  key={r.id}
                  rate={r}
                  deleteLabel={t("common:delete")}
                  onDelete={() => deleteRate.mutate(r.id)}
                />
              ))}
            </div>
          ) : (
            <Table data-testid="inflation-rate-table">
              <TableHeader>
                <TableRow>
                  <TableHead>{t("inflation.month")}</TableHead>
                  <TableHead>{t("inflation.rate")}</TableHead>
                  <TableHead className="w-16"></TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rates.map((r) => (
                  <TableRow key={r.id} data-testid="inflation-rate-row">
                    <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                    <TableCell className="tabular-nums" data-testid="inflation-rate-value">
                      {r.rate}
                    </TableCell>
                    <TableCell>
                      <Button variant="ghost" size="sm" onClick={() => deleteRate.mutate(r.id)}>
                        {t("common:delete")}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          ))}
      </CardContent>
    </Card>
  );
}

// Mobile leaf (ADR-0050 "wide table → stacked cards"): one card per inflation
// row. The rate — the annualized figure the user came to check — is promoted to
// the headline (a trailing "%" makes the unit explicit at a glance) with the
// month below. The delete control is an icon button sized to the 44px tap floor
// (INV-PRESENTATION-08).
function InflationRateCard({
  rate,
  deleteLabel,
  onDelete,
}: {
  rate: InflationRate;
  deleteLabel: string;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3" data-testid="inflation-rate-row">
      <div className="min-w-0 flex-1">
        <div className="text-lg font-semibold tabular-nums" data-testid="inflation-rate-value">
          {`${rate.rate}%`}
        </div>
        <div className="text-sm text-muted-foreground">{formatYearMonth(rate.year_month)}</div>
      </div>
      <Button
        variant="ghost"
        size="icon"
        className="size-11 shrink-0"
        aria-label={deleteLabel}
        onClick={onDelete}
      >
        <Trash2 className="size-4" />
      </Button>
    </div>
  );
}
