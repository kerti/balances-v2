import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumericInput } from "@/components/common/NumericInput";
import { formatCurrency, formatYearMonth, roundToCurrency, currencyDecimals } from "@/lib/format";
import type { RevaluationSuggestion } from "@/lib/revaluation";
import { SnapshotMonthRow } from "./SnapshotMonthRow";
import { LastRecordedHint, type LastRecorded } from "./LastRecordedHint";
import type { FormMode } from "./mode";

export type SnapshotFormState = {
  year_month: string;
  amount: string;
  as_of_date: string;
  description: string;
};

type Props = {
  mode: FormMode;
  currency: string;
  form: SnapshotFormState;
  setForm: (next: SnapshotFormState) => void;
  // Create only (property + vehicle, ADR-0008): the revaluation helper offered
  // under the amount. The Apply button is its only writer.
  suggest?: (yearMonth: string) => RevaluationSuggestion | null;
  // Create only (Record snapshot, #788). The dialog owns month switching so it
  // can prefill a replace; without it the month is a plain field.
  onMonthChange?: (yearMonth: string) => void;
  // The existing snapshot's formatted total when the picked month has one.
  replacing?: string;
  // The previous snapshot, for an empty month: the "Same as last month" line.
  lastRecorded?: LastRecorded;
  // Edit only: opens the Move dialog beside the locked month (#791).
  onMove?: () => void;
};

// Amount-only snapshot form body, shared by Record snapshot and Edit snapshot.
export function SnapshotFormBody({
  mode,
  currency,
  form,
  setForm,
  suggest,
  onMonthChange,
  replacing,
  lastRecorded,
  onMove,
}: Props) {
  const { t } = useTranslation("common");
  const s = mode === "create" ? suggest?.(form.year_month) : null;
  return (
    <>
      <SnapshotMonthRow
        mode={mode}
        idPrefix=""
        yearMonth={form.year_month}
        onYearMonthChange={onMonthChange ?? ((ym) => setForm({ ...form, year_month: ym }))}
        asOfDate={form.as_of_date}
        onAsOfDateChange={(v) => setForm({ ...form, as_of_date: v })}
        replacing={replacing}
        onMove={onMove}
      />

      <div className="grid gap-2">
        <Label htmlFor="amount">{t("fields.amountIn", { currency })}</Label>
        <NumericInput
          id="amount"
          required
          decimals={currencyDecimals(currency)}
          value={form.amount}
          onChange={(v) => setForm({ ...form, amount: v })}
          placeholder={t("snapshot.amountPlaceholder")}
        />
        {lastRecorded && <LastRecordedHint {...lastRecorded} />}
        {s && (
          <div
            className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground"
            data-testid="revaluation-hint"
          >
            {/* Sign-aware copy: positive rate reads as "appreciation", negative
                as "depreciation" (real minus U+2212 in EN, "penyusutan" in ID).
                Two keys keep the verb-form localisable. */}
            <span>
              {t(
                s.annualRatePct > 0
                  ? "snapshot.revaluationHintAppreciate"
                  : "snapshot.revaluationHintDepreciate",
                {
                  amount: formatCurrency(s.amount, currency),
                  magnitude: Math.abs(s.annualRatePct),
                  months: s.monthsElapsed,
                  anchor: formatYearMonth(s.anchorYearMonth + "-01T00:00:00Z"),
                },
              )}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 px-2"
              onClick={() =>
                // Round to the currency's display precision (0 dp for
                // IDR/JPY/KRW/VND, 2 dp elsewhere) so the input shows a clean
                // figure rather than the helper's raw 4dp.
                setForm({ ...form, amount: roundToCurrency(s.amount, currency) })
              }
              data-testid="revaluation-apply"
            >
              {t("snapshot.revaluationApply")}
            </Button>
          </div>
        )}
      </div>

      <div className="grid gap-2">
        <Label htmlFor="description">{t("fields.note")}</Label>
        <Input
          id="description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          placeholder={t("snapshot.descriptionPlaceholder")}
        />
      </div>
    </>
  );
}
