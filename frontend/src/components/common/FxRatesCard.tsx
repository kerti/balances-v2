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
import { useFxRates, useCreateFxRate, useDeleteFxRate } from "@/hooks/useFxRates";
import { formatYearMonth } from "@/lib/format";

// FxRatesCard is the Settings ▸ Exchange Rates subpage's sole content: the
// manual monthly FX table (ADR-0024). Only rendered when multi-currency is on
// (gated by the caller) — there is nothing to convert otherwise. No card-level
// title/description — the page header (SettingsFxRatesScreen) owns that.
export function FxRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useFxRates();
  const createRate = useCreateFxRate();
  const deleteRate = useDeleteFxRate();

  const [month, setMonth] = useState("");
  const [currency, setCurrency] = useState("");
  const [rate, setRate] = useState("");

  const add = () => {
    createRate.mutate(
      { year_month: month, currency: currency.toUpperCase(), rate },
      {
        onSuccess: () => {
          setMonth("");
          setCurrency("");
          setRate("");
        },
      },
    );
  };

  const canAdd = month !== "" && currency.length === 3 && rate !== "" && Number(rate) > 0;

  return (
    <Card data-testid="fx-rates-card">
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="fx-month">{t("fx.month")}</Label>
            <Input
              id="fx-month"
              type="month"
              className="w-40"
              value={month}
              onChange={(e) => setMonth(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="fx-currency">{t("fx.currency")}</Label>
            <Input
              id="fx-currency"
              className="w-24 uppercase"
              maxLength={3}
              // ISO currency code — a data token, not translatable copy.
              placeholder={"USD"}
              value={currency}
              onChange={(e) => setCurrency(e.target.value)}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="fx-rate">{t("fx.rate")}</Label>
            <Input
              id="fx-rate"
              inputMode="decimal"
              className="w-36"
              // Example numeric rate; not translatable copy.
              placeholder={"16000"}
              value={rate}
              onChange={(e) => setRate(e.target.value)}
            />
          </div>
          <Button onClick={add} disabled={!canAdd || createRate.isPending}>
            {t("fx.addRate")}
          </Button>
        </div>

        {createRate.isError && (
          <p className="text-sm text-destructive">{errorMessage(createRate.error)}</p>
        )}

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("fx.empty")}</p>
        )}

        {rates && rates.length > 0 && (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("fx.month")}</TableHead>
                <TableHead>{t("fx.currency")}</TableHead>
                <TableHead>{t("fx.rate")}</TableHead>
                <TableHead className="w-16"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {rates.map((r) => (
                <TableRow key={r.id}>
                  <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                  <TableCell>{r.currency}</TableCell>
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
