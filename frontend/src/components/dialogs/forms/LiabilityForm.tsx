import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { NumericInput } from "@/components/common/NumericInput";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import {
  RATE_DECIMALS,
  RATE_INTEGER_DIGITS,
  TERM_MONTHS_DIGITS,
  currencyDecimals,
} from "@/lib/format";
import type { EntryType } from "@/api/types";
import type { FormMode } from "./mode";

export type LiabilityFormState = {
  display_name: string;
  description: string;
  subtype: "personal" | "institutional";
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  counterparty_name: string;
  principal: string;
  interest_rate: string;
  term_months: string;
  start_date: string;
  maturity_date: string;
};

type Props = {
  mode: FormMode;
  form: LiabilityFormState;
  setForm: (next: LiabilityFormState) => void;
  soleOwnerID: string | null;
};

// Subtype and currency are fixed once the liability exists: Edit shows them
// disabled in the same slots as Create.
export function LiabilityFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["liabilities", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("liabilities:placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="subtype">{t("liabilities:fields.subtype")}</Label>
          <Select
            id="subtype"
            disabled={mode === "edit"}
            value={form.subtype}
            onChange={(e) =>
              setForm({ ...form, subtype: e.target.value as "personal" | "institutional" })
            }
          >
            <option value="personal">{t("liabilities:subtypes.personal")}</option>
            <option value="institutional">{t("liabilities:subtypes.institutional")}</option>
          </Select>
        </div>
        <div className="grid gap-2">
          <Label htmlFor="native_currency">{t("common:fields.currency")}</Label>
          <Input
            id="native_currency"
            required
            disabled={mode === "edit"}
            value={form.native_currency}
            onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
            placeholder={t("liabilities:placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="counterparty_name">{t("liabilities:fields.counterparty")}</Label>
        <Input
          id="counterparty_name"
          required
          value={form.counterparty_name}
          onChange={(e) => setForm({ ...form, counterparty_name: e.target.value })}
          placeholder={t("liabilities:placeholders.counterparty")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="principal">{t("liabilities:fields.principal")}</Label>
          <NumericInput
            id="principal"
            decimals={currencyDecimals(form.native_currency)}
            value={form.principal}
            onChange={(v) => setForm({ ...form, principal: v })}
            placeholder={t("liabilities:placeholders.principal")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="interest_rate">{t("liabilities:fields.interestRate")}</Label>
          <NumericInput
            id="interest_rate"
            decimals={RATE_DECIMALS}
            maxIntegerDigits={RATE_INTEGER_DIGITS}
            value={form.interest_rate}
            onChange={(v) => setForm({ ...form, interest_rate: v })}
            placeholder={t("liabilities:placeholders.interestRate")}
          />
        </div>
      </div>

      {/*
        Term / start date / end date. Three columns inside the dialog leaves
        ~100px each at 390px, which truncates the year off both date fields
        ("mm/dd/" with the yy cut) — a straight breach of the ADR-0050 bar, so
        this row stacks on phones. The two-column rows elsewhere in these forms
        deliberately do not: at ~157px they were verified to render the full
        date and its picker icon, and merely-cramped stays single-layout.
      */}
      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
        <div className="grid gap-2">
          <Label htmlFor="term_months">{t("liabilities:fields.term")}</Label>
          <NumericInput
            id="term_months"
            decimals={0}
            maxIntegerDigits={TERM_MONTHS_DIGITS}
            value={form.term_months}
            onChange={(v) => setForm({ ...form, term_months: v })}
            placeholder={t("liabilities:placeholders.term")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="start_date">{t("liabilities:fields.startDate")}</Label>
          <Input
            id="start_date"
            type="date"
            max="9999-12-31"
            value={form.start_date}
            onChange={(e) => setForm({ ...form, start_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="maturity_date">{t("liabilities:fields.maturityDate")}</Label>
          <Input
            id="maturity_date"
            type="date"
            max="9999-12-31"
            value={form.maturity_date}
            onChange={(e) => setForm({ ...form, maturity_date: e.target.value })}
          />
        </div>
      </div>

      <OwnershipField
        idPrefix={`liability_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`liability_${mode}`}
        group="liability"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="description">{t("common:fields.description")}</Label>
        <Input
          id="description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </>
  );
}
