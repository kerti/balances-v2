import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { NumericInput } from "@/components/common/NumericInput";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import {
  RATE_DECIMALS,
  RATE_INTEGER_DIGITS,
  TERM_MONTHS_DIGITS,
  currencyDecimals,
} from "@/lib/format";
import { addMonths, type TimeDepositForm } from "@/lib/rollover";
import type { RolloverPolicy } from "@/api/types";
import type { FormMode } from "./mode";

export type TimeDepositFormState = TimeDepositForm;

type Props = {
  mode: FormMode;
  form: TimeDepositFormState;
  setForm: (next: TimeDepositFormState) => void;
  soleOwnerID: string | null;
};

// Create derives maturity from placement + term (the user may nudge it by a
// day or two afterwards); Edit keeps the stored maturity until it is edited
// directly, so reopening a deposit never silently rewrites it.
export function TimeDepositFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  const derive = mode === "create";

  function setPlacement(v: string) {
    setForm({
      ...form,
      placement_date: v,
      maturity_date: derive
        ? addMonths(v, Number(form.term_months)) || form.maturity_date
        : form.maturity_date,
    });
  }

  function setTerm(v: string) {
    setForm({
      ...form,
      term_months: v,
      maturity_date: derive
        ? addMonths(form.placement_date, Number(v)) || form.maturity_date
        : form.maturity_date,
    });
  }

  return (
    <>
      <div className="space-y-3">
        <div className="grid gap-2">
          <Label htmlFor="td_display_name">{t("common:fields.displayName")}</Label>
          <Input
            id="td_display_name"
            required
            value={form.display_name}
            onChange={(e) => setForm({ ...form, display_name: e.target.value })}
            placeholder={t("investments:timeDeposit.placeholders.displayName")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="td_description">{t("common:fields.description")}</Label>
          <Input
            id="td_description"
            value={form.description}
            onChange={(e) => setForm({ ...form, description: e.target.value })}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="td_bank_name">{t("investments:timeDeposit.fields.bankName")}</Label>
            <Input
              id="td_bank_name"
              required
              value={form.bank_name}
              onChange={(e) => setForm({ ...form, bank_name: e.target.value })}
              placeholder={t("investments:timeDeposit.placeholders.bankName")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="td_currency">{t("common:fields.currency")}</Label>
            <Input
              id="td_currency"
              required
              disabled={mode === "edit"}
              value={form.native_currency}
              onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
              placeholder={t("investments:timeDeposit.placeholders.currency")}
              maxLength={3}
            />
          </div>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="td_principal">{t("investments:timeDeposit.fields.principal")}</Label>
          <NumericInput
            id="td_principal"
            required
            decimals={currencyDecimals(form.native_currency)}
            value={form.principal}
            onChange={(v) => setForm({ ...form, principal: v })}
            placeholder={t("investments:timeDeposit.placeholders.principal")}
          />
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="td_interest_rate">
              {t("investments:timeDeposit.fields.interestRate")}
            </Label>
            <NumericInput
              id="td_interest_rate"
              required
              decimals={RATE_DECIMALS}
              maxIntegerDigits={RATE_INTEGER_DIGITS}
              value={form.interest_rate}
              onChange={(v) => setForm({ ...form, interest_rate: v })}
              placeholder={t("investments:timeDeposit.placeholders.interestRate")}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="td_term_months">{t("investments:timeDeposit.fields.termMonths")}</Label>
            <NumericInput
              id="td_term_months"
              required
              decimals={0}
              maxIntegerDigits={TERM_MONTHS_DIGITS}
              value={form.term_months}
              onChange={(v) => setTerm(v)}
              placeholder={t("investments:timeDeposit.placeholders.termMonths")}
            />
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
          <div className="grid gap-2">
            <Label htmlFor="td_placement_date">
              {t("investments:timeDeposit.fields.placementDate")}
            </Label>
            <Input
              id="td_placement_date"
              required
              type="date"
              max="9999-12-31"
              value={form.placement_date}
              onChange={(e) => setPlacement(e.target.value)}
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="td_maturity_date">
              {t("investments:timeDeposit.fields.maturityDate")}
            </Label>
            <Input
              id="td_maturity_date"
              required
              type="date"
              min={form.placement_date || undefined}
              max="9999-12-31"
              value={form.maturity_date}
              onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
            />
          </div>
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <div className="grid gap-2">
          <Label htmlFor="td_rollover_policy">
            {t("investments:timeDeposit.fields.rolloverPolicy")}
          </Label>
          <Select
            id="td_rollover_policy"
            value={form.rollover_policy}
            onChange={(e) =>
              setForm({ ...form, rollover_policy: e.target.value as RolloverPolicy })
            }
          >
            <option value="auto_renew_principal">
              {t("investments:timeDeposit.rolloverPolicy.auto_renew_principal")}
            </option>
            <option value="auto_renew_with_interest">
              {t("investments:timeDeposit.rolloverPolicy.auto_renew_with_interest")}
            </option>
            <option value="no_rollover">
              {t("investments:timeDeposit.rolloverPolicy.no_rollover")}
            </option>
          </Select>
          <p className="text-xs text-muted-foreground">
            {t("investments:timeDeposit.rolloverHint")}
          </p>
        </div>
      </div>

      <div className="space-y-3 border-t pt-4">
        <OwnershipField
          idPrefix={`td_${mode}`}
          value={form.ownership_type}
          onChange={(v) => setForm({ ...form, ownership_type: v })}
          soleOwnerID={soleOwnerID}
          onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
        />

        <EntryTypeField
          idPrefix={`td_${mode}`}
          group="investment"
          value={form.entry_type}
          onChange={(v) => setForm({ ...form, entry_type: v })}
        />
      </div>

      <RiskProfileSelect
        idPrefix={`td_${mode}`}
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </>
  );
}
