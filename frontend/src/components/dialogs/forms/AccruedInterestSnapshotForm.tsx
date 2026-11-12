import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { NumericInput } from "@/components/common/NumericInput";
import { formatCurrency, currencyDecimals } from "@/lib/format";
import { timeDepositInterest } from "@/lib/timeDepositInterest";
import type { CouponDisposition } from "@/api/types";
import { SnapshotMonthRow } from "./SnapshotMonthRow";
import { LastRecordedHint, type LastRecorded } from "./LastRecordedHint";
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
  // Bond only: amount − accrued, derived by the dialog that submits it.
  derivedPrincipal?: string | null;
  // Bond coupon disposition (#66): picks the accrued-interest hint.
  couponDisposition?: CouponDisposition;
  // Time deposits only (#844): the principal in force. Its presence switches the
  // form to one Value input with a read-only "of which interest" line; the
  // server derives the stored accrued_interest from the same figure. Bonds pass
  // nothing and keep the total-value + accrued pair.
  principal?: string;
  // Create only (Record snapshot, #788); see SnapshotFormBody.
  onMonthChange?: (yearMonth: string) => void;
  replacing?: string;
  lastRecorded?: LastRecorded;
  // Edit only: opens the Move dialog beside the locked month (#791).
  onMove?: () => void;
};

export function AccruedInterestSnapshotFormBody({
  mode,
  currency,
  form,
  setForm,
  derivedPrincipal = null,
  couponDisposition,
  principal,
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
        idPrefix="ai_"
        yearMonth={form.year_month}
        onYearMonthChange={onMonthChange ?? ((ym) => setForm({ ...form, year_month: ym }))}
        asOfDate={form.as_of_date}
        onAsOfDateChange={(v) => setForm({ ...form, as_of_date: v })}
        replacing={replacing}
        onMove={onMove}
      />

      {principal !== undefined ? (
        <TimeDepositValue
          currency={currency}
          principal={principal}
          amount={form.amount}
          onAmountChange={(v) => setForm({ ...form, amount: v })}
          lastRecorded={lastRecorded}
        />
      ) : (
        <>
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

          {lastRecorded && <LastRecordedHint {...lastRecorded} />}

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
        </>
      )}

      <div className="grid gap-2">
        <Label htmlFor="ai_description">{t("common:fields.note")}</Label>
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

// A time deposit's snapshot: the one net figure the statement shows (#844). The
// interest beneath it is derived live from the principal in force; a Value below
// the principal is flagged softly and still saves.
function TimeDepositValue({
  currency,
  principal,
  amount,
  onAmountChange,
  lastRecorded,
}: {
  currency: string;
  principal: string;
  amount: string;
  onAmountChange: (v: string) => void;
  lastRecorded?: LastRecorded;
}) {
  const { t } = useTranslation("investments");
  const interest = timeDepositInterest(amount, principal);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="ai_amount">{t("accruedInterestSnapshot.valueLabel", { currency })}</Label>
        <NumericInput
          id="ai_amount"
          required
          decimals={currencyDecimals(currency)}
          value={amount}
          onChange={onAmountChange}
          placeholder={t("accruedInterestSnapshot.totalValuePlaceholder")}
        />
      </div>

      {lastRecorded && <LastRecordedHint {...lastRecorded} />}

      <div className="rounded-md bg-muted px-3 py-2 text-sm" data-testid="snapshot-interest">
        <span className="text-muted-foreground">
          {t("accruedInterestSnapshot.ofWhichInterestLabel")}
        </span>{" "}
        <span className="font-medium">
          {interest.valid ? formatCurrency(interest.amount, currency) : "—"}
        </span>
      </div>
      {interest.below && (
        <p className="text-xs text-warning" role="status" data-testid="snapshot-interest-warning">
          {t("accruedInterestSnapshot.belowPrincipal")}
        </p>
      )}
    </>
  );
}
