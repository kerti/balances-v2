import { useState } from "react";
import { useTranslation } from "react-i18next";
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
import { formatYearMonth } from "@/lib/format";

// InflationRatesCard is the Settings ▸ Inflation Rates subpage's sole content:
// the manual monthly table (ADR-0048's "FX-like store"). The assumed-annual
// fallback lives on the Settings home page (Household section) instead — it's
// a single household preference, not a row-based lookup table like this one.
// Rates are annualized (YoY) percentages and may be negative (deflation). No
// card-level title/description — the page header (SettingsInflationRatesScreen)
// owns that.
export function InflationRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useInflationRates();
  const createRate = useCreateInflationRate();
  const deleteRate = useDeleteInflationRate();

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

        {rates && rates.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("inflation.month")}</TableHead>
                <TableHead>{t("inflation.rate")}</TableHead>
                <TableHead className="w-16"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rates.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                  <TableCell className="tabular-nums">{r.rate}</TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm" onClick={() => deleteRate.mutate(r.id)}>
                      {t("common:delete")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
