import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumericInput } from "@/components/common/NumericInput";
import {
  formatCurrency,
  PRICE_DECIMALS,
  QUANTITY_DECIMALS,
  QUANTITY_INTEGER_DIGITS,
} from "@/lib/format";
import { SnapshotMonthRow } from "./SnapshotMonthRow";
import { LastRecordedHint, type LastRecorded } from "./LastRecordedHint";
import type { FormMode } from "./mode";

export type QuantityPriceSnapshotFormState = {
  year_month: string;
  quantity: string;
  price_per_unit: string;
  as_of_date: string;
  description: string;
};

type Props = {
  mode: FormMode;
  currency: string;
  form: QuantityPriceSnapshotFormState;
  setForm: (next: QuantityPriceSnapshotFormState) => void;
  // amount = quantity × price_per_unit, derived by the dialog that submits it.
  derivedAmount: string | null;
  // Gold's "use the buyback price" guidance under the price input (issue #19).
  priceHint?: string;
  // Create only (Record snapshot, #788); see SnapshotFormBody.
  onMonthChange?: (yearMonth: string) => void;
  replacing?: string;
  lastRecorded?: LastRecorded;
  // Edit only: opens the Move dialog beside the locked month (#791).
  onMove?: () => void;
};

export function QuantityPriceSnapshotFormBody({
  mode,
  currency,
  form,
  setForm,
  derivedAmount,
  priceHint,
  onMonthChange,
  replacing,
  lastRecorded,
  onMove,
}: Props) {
  const { t } = useTranslation(["investments", "common"]);
  return (
    <>
      <SnapshotMonthRow
        mode={mode}
        idPrefix="inv_"
        yearMonth={form.year_month}
        onYearMonthChange={onMonthChange ?? ((ym) => setForm({ ...form, year_month: ym }))}
        asOfDate={form.as_of_date}
        onAsOfDateChange={(v) => setForm({ ...form, as_of_date: v })}
        replacing={replacing}
        onMove={onMove}
      />

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="inv_quantity">
            {t("investments:quantityPriceSnapshot.quantityLabel")}
          </Label>
          <NumericInput
            id="inv_quantity"
            required
            decimals={QUANTITY_DECIMALS}
            maxIntegerDigits={QUANTITY_INTEGER_DIGITS}
            value={form.quantity}
            onChange={(v) => setForm({ ...form, quantity: v })}
            placeholder={t("investments:quantityPriceSnapshot.quantityPlaceholder")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="inv_price_per_unit">
            {t("investments:quantityPriceSnapshot.pricePerUnitLabel", { currency })}
          </Label>
          <NumericInput
            id="inv_price_per_unit"
            required
            decimals={PRICE_DECIMALS}
            value={form.price_per_unit}
            onChange={(v) => setForm({ ...form, price_per_unit: v })}
            placeholder={t("investments:quantityPriceSnapshot.pricePerUnitPlaceholder")}
          />
        </div>
      </div>

      {lastRecorded && <LastRecordedHint {...lastRecorded} />}

      {priceHint && <p className="text-xs text-muted-foreground">{priceHint}</p>}

      <div className="rounded-md bg-muted px-3 py-2 text-sm">
        <span className="text-muted-foreground">
          {t("investments:quantityPriceSnapshot.totalValueLabel")}
        </span>{" "}
        <span className="font-medium">
          {derivedAmount !== null ? formatCurrency(derivedAmount, currency) : "—"}
        </span>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="inv_snap_description">{t("common:fields.description")}</Label>
        <Input
          id="inv_snap_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          placeholder={t("investments:quantityPriceSnapshot.descriptionPlaceholder")}
        />
      </div>
    </>
  );
}
