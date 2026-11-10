import { useTranslation } from "react-i18next";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { NumericInput } from "@/components/common/NumericInput";
import { OwnershipField } from "@/components/common/OwnershipField";
import { EntryTypeField } from "@/components/common/EntryTypeField";
import { RATE_DECIMALS, RATE_INTEGER_DIGITS, currencyDecimals } from "@/lib/format";
import type { EntryType } from "@/api/types";
import type { FormMode } from "./mode";

export type PropertyFormState = {
  display_name: string;
  description: string;
  ownership_type: "sole" | "joint";
  entry_type: EntryType;
  sole_owner_user_id: string | null;
  native_currency: string;
  property_type: "house" | "apartment" | "land" | "commercial";
  address: string;
  acquisition_date: string;
  acquisition_cost: string;
  annual_appreciation_rate: string;
};

type Props = {
  mode: FormMode;
  form: PropertyFormState;
  setForm: (next: PropertyFormState) => void;
  soleOwnerID: string | null;
};

export function PropertyFormBody({ mode, form, setForm, soleOwnerID }: Props) {
  const { t } = useTranslation(["assets", "common"]);
  return (
    <>
      <div className="grid gap-2">
        <Label htmlFor="display_name">{t("common:fields.displayName")}</Label>
        <Input
          id="display_name"
          required
          value={form.display_name}
          onChange={(e) => setForm({ ...form, display_name: e.target.value })}
          placeholder={t("assets:property.placeholders.displayName")}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="property_type">{t("assets:property.fields.type")}</Label>
          <Select
            id="property_type"
            value={form.property_type}
            onChange={(e) =>
              setForm({ ...form, property_type: e.target.value as typeof form.property_type })
            }
          >
            <option value="house">{t("assets:property.propertyTypes.house")}</option>
            <option value="apartment">{t("assets:property.propertyTypes.apartment")}</option>
            <option value="land">{t("assets:property.propertyTypes.land")}</option>
            <option value="commercial">{t("assets:property.propertyTypes.commercial")}</option>
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
            placeholder={t("assets:property.placeholders.currency")}
            maxLength={3}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="address">{t("assets:property.fields.address")}</Label>
        <Input
          id="address"
          value={form.address}
          onChange={(e) => setForm({ ...form, address: e.target.value })}
        />
      </div>

      <div className="grid grid-cols-2 gap-3 [&>*]:content-end">
        <div className="grid gap-2">
          <Label htmlFor="acquisition_date">{t("assets:property.fields.acquisitionDate")}</Label>
          <Input
            id="acquisition_date"
            type="date"
            max="9999-12-31"
            value={form.acquisition_date}
            onChange={(e) => setForm({ ...form, acquisition_date: e.target.value })}
          />
        </div>
        <div className="grid gap-2">
          <Label htmlFor="acquisition_cost">{t("assets:property.fields.acquisitionCost")}</Label>
          <NumericInput
            id="acquisition_cost"
            decimals={currencyDecimals(form.native_currency)}
            value={form.acquisition_cost}
            onChange={(v) => setForm({ ...form, acquisition_cost: v })}
            placeholder={t("assets:property.placeholders.acquisitionCost")}
          />
        </div>
      </div>

      <div className="grid gap-2">
        <Label htmlFor="annual_appreciation_rate">
          {t("assets:property.fields.appreciationRate")}
        </Label>
        <NumericInput
          id="annual_appreciation_rate"
          decimals={RATE_DECIMALS}
          maxIntegerDigits={RATE_INTEGER_DIGITS}
          allowNegative
          value={form.annual_appreciation_rate}
          onChange={(v) => setForm({ ...form, annual_appreciation_rate: v })}
          placeholder={t("assets:property.placeholders.appreciationRate")}
        />
      </div>

      <OwnershipField
        idPrefix={`property_${mode}`}
        value={form.ownership_type}
        onChange={(v) => setForm({ ...form, ownership_type: v })}
        soleOwnerID={soleOwnerID}
        onSoleOwnerChange={(v) => setForm({ ...form, sole_owner_user_id: v })}
      />

      <EntryTypeField
        idPrefix={`property_${mode}`}
        group="asset"
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
