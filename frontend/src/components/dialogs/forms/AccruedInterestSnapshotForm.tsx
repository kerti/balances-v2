import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumericInput } from "@/components/common/NumericInput";
import { formatCurrency, currencyDecimals } from "@/lib/format";
import type { CouponDisposition } from "@/api/types";
import { SnapshotMonthRow } from "./SnapshotMonthRow";
import type { FormMode } from "./mode";

export type AccruedInterestSnapshotFormState = {
  year_month: string;
  amount: string;
  accrued_interest: string;
  as_of_date: string;
  description: string;
};

type Props = {
  mode: FormMode;
  currency: string;
  form: AccruedInterestSnapshotFormState;
  setForm: (next: AccruedInterestSnapshotFormState) => void;
  // amount − accrued, derived by the dialog that submits it.
  derivedPrincipal: string | null;
  // Bond coupon disposition (#66): picks the accrued-interest hint.
  couponDisposition?: CouponDisposition;
};

export function AccruedInterestSnapshotFormBody({
  mode,
  currency,
  form,
  setForm,
  derivedPrincipal,
  couponDisposition,
}: Props) {
  const { t } = useTranslation(["investments", "common"]);
  return (
    <>
      <SnapshotMonthRow
        mode={mode}
        idPrefix="ai_"
        yearMonth={form.year_month}
        onYearMonthChange={(ym) => setForm({ ...form, year_month: ym })}
        asOfDate={form.as_of_date}
        onAsOfDateChange={(v) => setForm({ ...form, as_of_date: v })}
      />

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="ai_amount">
            {t("investments:accruedInterestSnapshot.totalValueLabel", { currency })}
          </Label>
          <NumericInput
            id="ai_amount"
            required
            decimals={currencyDecimals(currency)}
            value={form.amount}
            onChange={(v) => setForm({ ...form, amount: v })}
            placeholder={t("investments:accruedInterestSnapshot.totalValuePlaceholder")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="ai_accrued">
            {t("investments:accruedInterestSnapshot.accruedLabel", { currency })}
          </Label>
          <NumericInput
            id="ai_accrued"
            required
            decimals={currencyDecimals(currency)}
            value={form.accrued_interest}
            onChange={(v) => setForm({ ...form, accrued_interest: v })}
            placeholder={t("investments:accruedInterestSnapshot.accruedPlaceholder")}
          />
        </div>
      </div>

      <div className="rounded-md bg-muted px-3 py-2 text-sm">
        <span className="text-muted-foreground">
          {t("investments:accruedInterestSnapshot.ofWhichPrincipalLabel")}
        </span>{" "}
        <span className="font-medium">
          {derivedPrincipal !== null ? formatCurrency(derivedPrincipal, currency) : "—"}
        </span>
      </div>

      <p className="text-xs text-muted-foreground">
        {couponDisposition === "accrues"
          ? t("investments:accruedInterestSnapshot.accruedHintAccrues")
          : t("investments:accruedInterestSnapshot.accruedHint")}
      </p>

      <div className="grid gap-2">
        <Label htmlFor="ai_description">{t("common:fields.description")}</Label>
        <Input
          id="ai_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          placeholder={t("investments:accruedInterestSnapshot.descriptionPlaceholder")}
        />
      </div>
    </>
  );
}
