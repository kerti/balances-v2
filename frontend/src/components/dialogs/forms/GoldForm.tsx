import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { GoldPuritySelect } from "@/components/common/GoldPuritySelect";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { GoldForm } from "@/hooks/useInvestments";
import type { EntryType, RiskProfile } from "@/api/types";
import type { FormMode } from "./mode";

export type GoldFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile | "";
  native_currency: string;
  form: GoldForm;
  purity: string;
};

type Props = {
  mode: FormMode;
  form: GoldFormState;
  setForm: (next: GoldFormState) => void;
  soleOwnerID: string | null;
};

export function GoldFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="gold_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="gold_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("investments:gold.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="gold_form">{t("investments:gold.fields.form")}</Label>
          <Select
            id="gold_form"
            value={form.form}
            onChange={(e) => setForm({ ...form, form: e.target.value as GoldForm })}
          >
            <option value="bar">{t("investments:gold.goldForms.bar")}</option>
            <option value="coin">{t("investments:gold.goldForms.coin")}</option>
            <option value="digital">{t("investments:gold.goldForms.digital")}</option>
            <option value="jewelry">{t("investments:gold.goldForms.jewelry")}</option>
          </Select>
        </div>
        <GoldPuritySelect
          idPrefix={`gold_${mode}`}
          value={form.purity}
          onChange={(v) => setForm({ ...form, purity: v })}
        />
      </div>

      <div className="grid gap-2">
        <Label htmlFor="gold_currency">{t("common:fields.currency")}</Label>
        <Input
          id="gold_currency"
          required
          disabled={mode === "edit"}
          value={form.native_currency}
          onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
          placeholder={t("investments:gold.placeholders.currency")}
          maxLength={3}
        />
      </div>

      <OwnershipField
        idPrefix={`gold_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`gold_${mode}`}
        group="investment"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="gold_description">{t("common:fields.description")}</Label>
        <Input
          id="gold_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix={`gold_${mode}`}
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </>
  );
}
