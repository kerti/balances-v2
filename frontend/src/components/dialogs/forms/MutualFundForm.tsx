import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { MutualFundTypeSelect } from "@/components/common/MutualFundTypeSelect";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType, MutualFundType, RiskProfile } from "@/api/types";
import type { FormMode } from "./mode";
import { LockedFieldHint } from "./LockedFieldHint";

export type MutualFundFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  risk_profile: RiskProfile | "";
  native_currency: string;
  fund_code: string;
  fund_manager: string;
  fund_type: MutualFundType | "";
};

type Props = {
  mode: FormMode;
  form: MutualFundFormState;
  setForm: (next: MutualFundFormState) => void;
  soleOwnerID: string | null;
};

export function MutualFundFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="mf_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="mf_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("investments:mutualFund.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="mf_fund_code">{t("investments:mutualFund.fields.fundCode")}</Label>
          <Input
            id="mf_fund_code"
            required
            value={form.fund_code}
            onChange={(e) => setForm({ ...form, fund_code: e.target.value })}
            placeholder={t("investments:mutualFund.placeholders.fundCode")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="mf_fund_manager">{t("investments:mutualFund.fields.fundManager")}</Label>
          <Input
            id="mf_fund_manager"
            value={form.fund_manager}
            onChange={(e) => setForm({ ...form, fund_manager: e.target.value })}
            placeholder={t("investments:mutualFund.placeholders.fundManager")}
          />
        </div>
      </div>

      <MutualFundTypeSelect
        idPrefix={`mf_${mode}`}
        value={form.fund_type}
        onChange={(v) => setForm({ ...form, fund_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="mf_currency">{t("common:fields.currency")}</Label>
        <LockedFieldHint mode={mode} id="mf_currency" />
        <Input
          id="mf_currency"
          required
          disabled={mode === "edit"}
          value={form.native_currency}
          onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
          placeholder={t("investments:mutualFund.placeholders.currency")}
          maxLength={3}
        />
      </div>

      <OwnershipField
        idPrefix={`mf_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`mf_${mode}`}
        group="investment"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="mf_description">{t("common:fields.description")}</Label>
        <Input
          id="mf_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>

      <RiskProfileSelect
        idPrefix={`mf_${mode}`}
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />
    </>
  );
}
