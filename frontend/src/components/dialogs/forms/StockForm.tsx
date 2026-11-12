import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { RiskProfileSelect } from "@/components/common/RiskProfileSelect";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import type { EntryType, RiskProfile } from "@/api/types";
import type { FormMode } from "./mode";
import { LockedFieldHint } from "./LockedFieldHint";

export type StockFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  risk_profile: RiskProfile | "";
  ticker: string;
  exchange: string;
};

type Props = {
  mode: FormMode;
  form: StockFormState;
  setForm: (next: StockFormState) => void;
  soleOwnerID: string | null;
};

export function StockFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["investments", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="stock_display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="stock_display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("investments:stock.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="stock_ticker">{t("investments:stock.fields.ticker")}</Label>
          <Input
            id="stock_ticker"
            required
            value={form.ticker}
            onChange={(e) => setForm({ ...form, ticker: e.target.value.toUpperCase() })}
            placeholder={t("investments:stock.placeholders.ticker")}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="stock_exchange">{t("investments:stock.fields.exchange")}</Label>
          <Input
            id="stock_exchange"
            required
            value={form.exchange}
            onChange={(e) => setForm({ ...form, exchange: e.target.value.toUpperCase() })}
            placeholder={t("investments:stock.placeholders.exchange")}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="stock_currency">{t("common:fields.currency")}</Label>
        <LockedFieldHint mode={mode} id="stock_currency" />
        <Input
          id="stock_currency"
          required
          disabled={mode === "edit"}
          value={form.native_currency}
          onChange={(e) => setForm({ ...form, native_currency: e.target.value.toUpperCase() })}
          placeholder={t("investments:stock.placeholders.currency")}
          maxLength={3}
        />
      </div>

      <OwnershipField
        idPrefix={`stock_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`stock_${mode}`}
        group="investment"
        value={form.entry_type}
        onChange={(v) => setForm({ ...form, entry_type: v })}
      />

      <RiskProfileSelect
        idPrefix={`stock_${mode}`}
        value={form.risk_profile}
        onChange={(v) => setForm({ ...form, risk_profile: v })}
      />

      <div className="grid gap-2">
        <Label htmlFor="stock_description">{t("common:fields.description")}</Label>
        <Input
          id="stock_description"
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
        />
      </div>
    </>
  );
}
