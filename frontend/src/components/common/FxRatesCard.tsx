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
import { useFxRates, useCreateFxRate, useDeleteFxRate } from "@/hooks/useFxRates";
import { useSession } from "@/hooks/useSession";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatYearMonth } from "@/lib/format";
import type { FxRate } from "@/api/types";

// The desktop **Currency** column names the whole pair — the foreign currency
// being priced and the reporting currency it converts to (ADR-0002: `rate` is
// reporting-currency units per 1 unit of the foreign currency). A bare "USD"
// hides both the base and the direction; "USD → IDR" reads as "USD converts to
// IDR" under a labelled column, with the number safely in its own **Rate**
// column. Falls back to the bare code until the session (so the reporting
// currency) has loaded.
function fxPair(currency: string, base: string | undefined): string {
  return base ? `${currency} → ${base}` : currency;
}

// The mobile card and the add-form hint spell the rate as a full **equation**
// (`1 USD = 15600 IDR`) rather than a bare number beside the pair — on a stacked
// card the promoted number would sit flush against "USD → IDR" and misread as
// "15600 USD". The equation binds the number to the *reporting* currency, which
// is what it actually is. `rate` may be "?" in the live add-form hint before a
// value is typed. Built as an expression to stay clear of the ADR-0026
// bare-JSX-text lint.
function fxEquation(currency: string, rate: string, base: string | undefined): string {
  return base ? `1 ${currency} = ${rate} ${base}` : `1 ${currency} = ${rate}`;
}

// FxRatesCard is the Settings ▸ Exchange Rates subpage's sole content: the
// manual monthly FX table (ADR-0024). Only rendered when multi-currency is on
// (gated by the caller) — there is nothing to convert otherwise. No card-level
// title/description — the page header (SettingsFxRatesScreen) owns that.
//
// Mobile–web layout divergence (ADR-0050 B3b, #511): the add form is shared, but
// the entered rows split at the renderer — `useIsMobile` (768px) mounts stacked
// cards on phones and the wide table on desktop, so only one tree is ever in the
// DOM. Both leaves are fed the same rows and share the `fx-rate-row` testid.
export function FxRatesCard() {
  const { t } = useTranslation(["settings", "common"]);
  const { data: rates, isPending } = useFxRates();
  const { data: me } = useSession();
  const createRate = useCreateFxRate();
  const deleteRate = useDeleteFxRate();
  const isMobile = useIsMobile();

  const base = me?.reporting_currency?.toUpperCase();

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

        {/* Live direction hint: once a currency code is entered, spell out which
            way the rate converts and into which currency, so the form isn't a
            bare foreign-code field with no counterpart. */}
        {base && currency.length === 3 && (
          <p className="text-sm text-muted-foreground" data-testid="fx-rate-hint">
            {fxEquation(currency.toUpperCase(), rate.trim() || "?", base)}
          </p>
        )}

        {createRate.isError && (
          <p className="text-sm text-destructive">{errorMessage(createRate.error)}</p>
        )}

        {isPending && <p className="text-sm text-muted-foreground">{t("common:loading")}</p>}

        {rates && rates.length === 0 && (
          <p className="text-sm text-muted-foreground">{t("fx.empty")}</p>
        )}

        {rates &&
          rates.length > 0 &&
          (isMobile ? (
            <div className="space-y-2" data-testid="fx-rate-cards">
              {rates.map((r) => (
                <FxRateCard
                  key={r.id}
                  rate={r}
                  base={base}
                  deleteLabel={t("common:delete")}
                  onDelete={() => deleteRate.mutate(r.id)}
                />
              ))}
            </div>
          ) : (
            <Table data-testid="fx-rate-table">
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
                  <TableRow key={r.id} data-testid="fx-rate-row">
                    <TableCell>{formatYearMonth(r.year_month)}</TableCell>
                    <TableCell>{fxPair(r.currency, base)}</TableCell>
                    <TableCell className="tabular-nums" data-testid="fx-rate-value">
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

// Mobile leaf (ADR-0050 "wide table → stacked cards"): one card per FX row. The
// rate is promoted to the headline as a full **equation** (`1 USD = 15600 IDR`)
// — a bare number beside "USD → IDR" would misread as "15600 USD"; the equation
// binds the value to the reporting currency it actually is. The month sits
// below. The delete control is an icon button sized to the 44px tap floor
// (INV-PRESENTATION-08).
function FxRateCard({
  rate,
  base,
  deleteLabel,
  onDelete,
}: {
  rate: FxRate;
  base: string | undefined;
  deleteLabel: string;
  onDelete: () => void;
}) {
  return (
    <div className="flex items-center gap-3 rounded-lg border p-3" data-testid="fx-rate-row">
      <div className="min-w-0 flex-1">
        <div className="text-lg font-semibold tabular-nums" data-testid="fx-rate-value">
          {fxEquation(rate.currency, rate.rate, base)}
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
